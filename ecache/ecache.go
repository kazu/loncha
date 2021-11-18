package ecache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

type CacheType byte

type Record interface {
	list_head.List
	CacheKey() string
}

const (
	TypeNone CacheType = 0
	TypeLRU  CacheType = 1
	TypeLFU  CacheType = 2
)

const (
	MaxOfDirtHandler = 10
	MaxOfGc          = 1
)

var (
	ErrorNotDefinedKeyFn error = errors.New("key function is not defined")
	ErrorNotFound        error = errors.New("cache not found")
)

const MaxUint64 uint64 = 18446744073709551615

const ValidateOnUpdate = false

type Cache struct {
	cType CacheType
	cnt   uint64
	max   uint64
	sync.RWMutex
	key2CacheHead map[string]*list_head.ListHead
	start         *list_head.ListHead
	last          *list_head.ListHead
	unused        CachePool
	cntOfunused   uint64
	empty         CacheEntry

	dirties           chan reqStore
	cntOfDirtyHandler int32

	gcMu        sync.Mutex
	gcCtx       context.Context
	gcCtxCancel context.CancelFunc
	gcCh        chan bool
	gcWg        *sync.WaitGroup
	allocFn     func() CacheEntry
}

type reqfUpdateList struct {
	k     string
	hit   *list_head.ListHead
	found bool
}

type reqStore struct {
	record Record
	append bool
	stop   bool
}

type Opt func(c *Cache)

func New(opts ...Opt) (c *Cache) {

	list_head.MODE_CONCURRENT = true

	c = &Cache{
		cType:         TypeNone,
		max:           MaxUint64,
		key2CacheHead: map[string]*list_head.ListHead{},
		unused: &syncPool{
			New: func() interface{} { return new(CacheHead) },
		},
		dirties:           make(chan reqStore, MaxOfDirtHandler*10),
		cntOfDirtyHandler: 0,
		gcCh:              make(chan bool, 4),
	}

	c.start = &list_head.ListHead{}
	c.start.InitAsEmpty()
	c.start = c.start.Prev()
	c.last = c.start.Next()

	//c.unused = &syncPool{}

	for _, opt := range opts {
		opt(c)
	}

	c.gcWg = &sync.WaitGroup{}
	go c.gc()
	return
}

func Max(m int) Opt {

	return func(c *Cache) {
		c.max = uint64(m)
		//c.dirties = make(chan *list_head.ListHead, m/2)
	}
}

func PoolFn(fn func() interface{}) Opt {

	return func(c *Cache) {
		c.unused = &syncPool{New: fn}
	}
}

func AllocFn(fn func() CacheEntry) Opt {

	return func(c *Cache) {
		c.allocFn = fn
	}
}

func UseListPool() Opt {

	return func(c *Cache) {
		c.unused = &listPool{c: c}
	}

}

func Sample(r CacheEntry) Opt {

	return func(c *Cache) {
		c.empty = r
	}
}

func optalgorithm(t CacheType) Opt {
	return func(c *Cache) {
		c.cType = t
	}
}

func LRU() Opt {
	return optalgorithm(TypeLRU)
}
func LFU() Opt {
	return optalgorithm(TypeLFU)
}

func (c *Cache) Size() int {
	return len(c.key2CacheHead)
}

func (c *Cache) Keys() (result []string) {
	result = make([]string, 0, c.Size())

	for k, _ := range c.key2CacheHead {
		result = append(result, k)
	}
	return
}

func (c *Cache) ReverseEach(fn func(CacheEntry)) {
	for cur := c.last.Prev(list_head.WaitNoM()); !cur.Empty(); cur = cur.Prev(list_head.WaitNoM()) {
		r := c.DataFromListead(cur)
		if !r.isRegister() {
			continue
		}
		fn(r)
	}
	return
}

func (c *Cache) GetByPool() (head *list_head.ListHead) {
	c.cntOfunused--
	v := c.unused.get()
	head = v.PtrListHead()
	return
}

func (c *Cache) PushToPool(head *list_head.ListHead) {
	c.cntOfunused++
	c.cnt--
	v := EmptyCacheHead.fromListHead(head)
	c.unused.put(v)
	return
}

func (c *Cache) startIsFront() bool {
	if c.start == nil {
		return true
	}
	if !c.start.IsFirst() {
		return false
	}
	return true

}

func (c *Cache) Reset() {

	c.key2CacheHead = map[string]*list_head.ListHead{}
	if c.start == nil {
		return
	}

	if _, ok := c.unused.(*listPool); ok {

		c.gcCtxCancel()
		//c.gcWg.Wait()
		c.gcMu.Lock()
		c.gcMu.Unlock()
		c.gcCh <- false

		for l := c.start.Front(); !l.Empty(); l = l.Next(list_head.WaitNoM()) {
			chead := EmptyCacheHead.fromListHead(l)
			chead.expire()
		}
		last := c.last.Prev(list_head.WaitNoM())
		if last.IsLast() {
			chead := EmptyCacheHead.fromListHead(last)
			_ = chead
		}

		c.cntOfunused += c.cnt
		c.cnt = 0
		go c.gc()

		return
	}

	return

}

func (c *Cache) handleDirties() {
	if atomic.LoadInt32(&c.cntOfDirtyHandler) >= MaxOfDirtHandler {
		return
	}

	atomic.AddInt32(&c.cntOfDirtyHandler, 1)

	for dirty := range c.dirties {
		if dirty.stop {
			break
		}
		c.setLazy(dirty)
	}
	atomic.AddInt32(&c.cntOfDirtyHandler, -1)

}

func (c *Cache) Set(r Record) error {

	return c.set(r, true)
}
func (c *Cache) set(r Record, append bool) error {

	c.dirties <- reqStore{record: r, append: append, stop: false}
	if atomic.LoadInt32(&c.cntOfDirtyHandler) < MaxOfDirtHandler {
		go c.handleDirties()
	}

	return nil

}

func (c *Cache) SetFn(updateFn func(*list_head.ListHead) CacheEntry) error {

	nr := updateFn(c.empty.PtrListHead())
	k := nr.CacheKey()
	c.RLock()
	nHead, found := c.key2CacheHead[k]
	c.RUnlock()
	if !found {
		nHead = c.GetByPool()
	}
	// else {
	// 	nHead.Delete()
	// }
	updateFn(nHead)
	nRec := c.empty.FromListHead(nHead).(Record)
	c.set(nRec, false)
	return nil

}

func WaitNoM() list_head.TravOpt {

	return list_head.WaitNoM()
}

func (c *Cache) ValidateReverse() error {

	for cur, prev := c.last.Prev(WaitNoM()), c.last.Prev(WaitNoM()).Prev(WaitNoM()); !prev.Empty(); cur, prev = prev, prev.Prev(WaitNoM()) {

		err := list_head.Retry(10, func(retry int) (exit bool, err error) {

			if prev.Next(WaitNoM()) != cur {
				prev = cur.Prev(WaitNoM())
				return false, errors.New("prev.next != cur")
			}
			if cur.Prev(WaitNoM()) != prev {
				prev = cur.Prev(WaitNoM())
				return false, errors.New("cur.prev != prev")
			}
			return true, nil
		})
		if err != nil {
			return err
		}
	}
	return nil

}

func (c *Cache) ValidateTiny() error {

	return list_head.Retry(100, func(retry int) (done bool, err error) {
		sb := c.start.Back()
		sbn := sb.Next()
		sbn2 := sb.Next()
		_ = sbn2
		if !sbn.Empty() {
			return false, errors.New("not last")
		}

		if sbn.Empty() && c.last.Empty() && sbn != c.last {
			lfp := c.last.Front().Prev()
			_ = lfp
			err := c.ValidateReverse()
			_ = err
			return false,
				fmt.Errorf("c.start.Back().Next()[%p] != c.last[%p] ",
					sbn, c.last)

		}
		if sbn != c.last {
			return false,
				fmt.Errorf("c.start.Back().Next()[%p] != c.last[%p] ",
					sbn, c.last)
			//errors.New("c.start.Back().Next() != c.last")
		}
		return true, nil
	})

}

func (c *Cache) Validation() error {

	lf := c.last.Front()
	sf := c.start.Front()
	lfb := lf.Back()
	sfb := sf.Back()
	_, _, _, _ = lf, sf, lfb, sfb

	err := c.ValidateTiny()
	if err != nil {
		return err
	}

	for {
		if c.start == nil {
			return nil
		}

		if err := c.start.Front().Validate(); err != nil && err.Error() == "list not first element" {
			continue
		} else if err != nil {
			c.start.Front().Validate()
			return err
		}
		break
	}
	return nil
}

func (c *Cache) gc() {

	//if c.gcCtx == nil {
	c.gcCtx, c.gcCtxCancel = context.WithCancel(context.Background())
	//}

	for t := range c.gcCh {
		if !t {
			return
		}
		c._gc(c.gcCtx)
	}
}

func (c *Cache) _gc(ctx context.Context) {

	c.gcMu.Lock()
	defer c.gcMu.Unlock()

	if c.cnt+c.cntOfunused < c.max*2 {
		return
	}
	if c.last == nil {
		return
	}

	if c.start == nil {
		return
	}

	// gc
	for cur, prev := c.last.Prev(list_head.WaitNoM()), c.last.Prev(list_head.WaitNoM()).Prev(list_head.WaitNoM()); !prev.IsFirst(); cur, prev = prev, prev.Prev(list_head.WaitNoM()) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		//chead := EmptyCacheHead.fromListHead(cur)
		cEntry := c.empty.FromListHead(cur).(CacheEntry)

		if cEntry.cntOfRef() == 0 {
			if cEntry.isRegister() {
				c.Lock()
				delete(c.key2CacheHead, cEntry.CacheKey())
				c.Unlock()
				c.cnt--

			}
			c.unused.put(cEntry.PtrCacheHead())
			c.cntOfunused++
		}
		if c.cnt+c.cntOfunused < c.max*2 {
			break
		}
	}

	// goto second chanse
	for cur, next := c.start.Front(), c.start.Front().Next(list_head.WaitNoM()); !next.Empty(); cur, next = next, next.Next(list_head.WaitNoM()) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// METNION: dont gc first element. becasuse first element delettion is heavy cost.
		if cur.IsFirst() {
			continue
		}

		//cEntry := c.empty.FromListHead(cur).(CacheEntry)
		chead := EmptyCacheHead.fromListHead(cur)
		if c.cType == TypeLFU {
			atomic.AddInt32(&chead.reference, -1)
			if chead.reference <= 0 {
				//chead.Delete()
				c.DeleteAndCheck(&chead.ListHead)
				c.last = c.addLast(&chead.ListHead)
			}
			continue
		}

		if c.cType == TypeLRU {
			if chead.cntOfRef() <= 0 {
				c.DeleteAndCheck(&chead.ListHead)
				//chead.Delete()
				c.last = c.addLast(&chead.ListHead)
				continue
			}
			if chead.cntOfRef() > 0 {
				atomic.StoreInt32(&chead.reference, 0)
			}
			continue
		}

	}
	return
}

func purgeResult(active *list_head.ListHead, purged *list_head.ListHead) (bool, *list_head.ListHead, *list_head.ListHead) {

	if purged == nil {
		return false, active, purged
	}
	return true, active, purged

}

func (c *Cache) DeleteAndCheck(l *list_head.ListHead) *list_head.ListHead {

	success, active, purged := purgeResult(l.Purge())
	_ = active
	if !success {
		fmt.Printf("fail purge")
	}
	if ValidateOnUpdate {
		err := c.ValidateTiny()
		if err != nil {
			fmt.Printf("fail valdiate err=%s", err)
		}

	}

	return purged

}

func (c *Cache) addLast(l *list_head.ListHead) *list_head.ListHead {

	if l.IsMarked() {
		_ = c
	}
	isSingle := l.IsSingle()
	_ = isSingle
	if !isSingle {
		fmt.Printf("not single node")
	}
	resultLast, err := c.last.InsertBefore(l)
	if err != nil || resultLast != c.last {
		_ = "???"
	}
	return c.last

}

func (c *Cache) setLazy(req reqStore) error {
	v := req.record

	vhead := v.PtrListHead()
	if req.append {
		vhead.Init()
	}

	k := v.CacheKey()

	c.RLock()
	hit, found := c.key2CacheHead[k]
	c.RUnlock()

	defer func() {
		if len(c.gcCh) == 0 {
			c.gcCh <- true
		}
	}()

	var hhead *CacheHead

	if found && hit == v.PtrListHead() {
		c.updateInfoOnAdd(hit)
		goto AFTER
	}
	if found && hit != nil {
		c.PushToPool(hit)
	}
	if !found && c.start != nil {
		if req.append {
			c.last = c.addLast(vhead)
		}
		c.cnt++
	}

	hit = vhead

	if c.start != nil {
		c.updateInfoOnAdd(hit)
	}

	c.Lock()
	c.key2CacheHead[k] = hit
	hhead = EmptyCacheHead.fromListHead(hit)
	if !hhead.isRegister() {
		c.cnt++
	}
	hhead.regist(true)
	c.Unlock()

	c.startIsFront()

AFTER:

	return nil

}
func (c *Cache) updateInfoOnAdd(hit *list_head.ListHead) {

	switch c.cType {
	case TypeLFU:
		chead := EmptyCacheHead.fromListHead(hit)
		chead.referenced()
		break
	case TypeLRU:
		chead := EmptyCacheHead.fromListHead(hit)
		if chead.cntOfRef() != 2 {
			chead.reference = 2
		}
	}
}

type Fetcher func(head *list_head.ListHead)

func (c *Cache) Fetch(k string, fn Fetcher) error {

	c.RLock()
	hit, found := c.key2CacheHead[k]
	c.RUnlock()

	if !found {
		return ErrorNotFound
	}

	if c.cType != TypeLRU && c.cType != TypeLFU {
		fn(hit)
		return nil
	}
	fn(hit)

	c.updateList(k, hit, found)

	return nil
}

func (c *Cache) DataFromListead(head *list_head.ListHead) (r CacheEntry) {
	data := c.empty.FromListHead(head)
	succ := false
	if data == nil {
		goto FAIL
	}
	r, succ = data.(CacheEntry)
	if !succ {
		goto FAIL
	}
	return

FAIL:
	r = nil
	return r
}
func (c *Cache) GetHead(k string) (hit *list_head.ListHead) {

	c.RLock()
	found := false
	hit, found = c.key2CacheHead[k]
	c.RUnlock()
	if !found {
		return nil
	}
	return nil

}

func (c *Cache) Get(k string) (r CacheEntry, e error) {

	c.RLock()
	hit, found := c.key2CacheHead[k]
	c.RUnlock()

	//return r, ErrorNotFound

	if !found {
		return r, ErrorNotFound
	}
	r = c.DataFromListead(hit)
	if c.cType != TypeLRU && c.cType != TypeLFU {
		return
	}

	c.updateList(k, hit, found)
	//go c.PushReqOfUpdateList(k, hit, found)

	return r, nil
}

func (c *Cache) updateList(k string, hit *list_head.ListHead, found bool) {

	if found {
		switch c.cType {
		case TypeLFU:
			chead := EmptyCacheHead.fromListHead(hit)
			chead.referenced()
			break
		case TypeLRU:
			chead := EmptyCacheHead.fromListHead(hit)
			if chead.cntOfRef() != 2 {
				chead.reference = 2
			}
			break
		}
	}

	if !found {
		chead := EmptyCacheHead.fromListHead(hit)
		if !chead.isRegister() {
			c.cnt++
		}
		chead.regist(true)
		c.Lock()
		c.key2CacheHead[k] = hit
		c.Unlock()

	}

}
