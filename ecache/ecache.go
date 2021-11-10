package ecache

import (
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
)

var (
	ErrorNotDefinedKeyFn error = errors.New("key function is not defined")
	ErrorNotFound        error = errors.New("cache not found")
)

const MaxUint64 uint64 = 18446744073709551615

type Cache struct {
	cType CacheType
	cnt   uint64
	max   uint64
	sync.RWMutex
	key2ListHead map[string]*list_head.ListHead
	start        *list_head.ListHead
	last         *list_head.ListHead
	unused       sync.Pool
	cntOfunused  uint64
	empty        Record

	dirties           chan *list_head.ListHead
	cntOfDirtyHandler int32

	reqPool         sync.Pool
	reqUList        chan *reqfUpdateList
	cntOfReqHandler int32

	startMu sync.RWMutex
}

type reqfUpdateList struct {
	k     string
	hit   *list_head.ListHead
	found bool
}

type Opt func(c *Cache)

func New(opts ...Opt) (c *Cache) {
	c = &Cache{
		cType:        TypeNone,
		max:          MaxUint64,
		key2ListHead: map[string]*list_head.ListHead{},
		unused: sync.Pool{
			New: func() interface{} { return new(list_head.ListHead) },
		},
		dirties:           make(chan *list_head.ListHead, MaxOfDirtHandler*10),
		cntOfDirtyHandler: 0,
		reqUList:          make(chan *reqfUpdateList, MaxOfDirtHandler*1000),
		cntOfReqHandler:   0,
		reqPool: sync.Pool{
			New: func() interface{} { return new(reqfUpdateList) },
		},
	}
	for _, opt := range opts {
		opt(c)
	}

	list_head.MODE_CONCURRENT = false

	return
}

func Max(m int) Opt {

	return func(c *Cache) {
		c.max = uint64(m)
		//c.dirties = make(chan *list_head.ListHead, m/2)
	}
}

func AllocFn(fn func() interface{}) Opt {

	return func(c *Cache) {
		c.unused = sync.Pool{New: fn}
	}

}

func Sample(r Record) Opt {

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

func (c *Cache) GetByPool() (head *list_head.ListHead) {
	c.cntOfunused--
	v := c.unused.Get().(Record)
	head = v.PtrListHead()
	return
}

func (c *Cache) PushToPool(head *list_head.ListHead) {
	c.cntOfunused++
	if v, ok := c.empty.FromListHead(head).(Record); ok {
		c.unused.Put(v)
		return
	}
	c.unused.Put(head)
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

	c.key2ListHead = map[string]*list_head.ListHead{}
	if c.start == nil {
		return
	}

	for {
		if c.start == c.start.Next() {
			c.PushToPool(c.start)
			c.startMu.Lock()
			c.start.Delete()
			c.start = nil
			c.startMu.Unlock()
			break
		}
		c.startMu.Lock()

		expire := c.start.Front()
		c.start = c.start.Front().Next()
		expire.Delete()

		c.startIsFront()
		c.startMu.Unlock()

		c.PushToPool(expire)
	}

	return

}

// func (c *Cache) moveUnused(k string) error {
// 	hit, found := c.key2ListHead[k]
// 	if !found {
// 		return ErrorNotFound
// 	}
// 	c.Lock()
// 	c.key2ListHead[k] = nil
// 	c.Unlock()
// 	hit.Delete()
// 	c.PushToPool(hit)
// 	return nil
// }

func (c *Cache) handleDirties() {
	if atomic.LoadInt32(&c.cntOfDirtyHandler) >= MaxOfDirtHandler {
		return
	}

	atomic.AddInt32(&c.cntOfDirtyHandler, 1)

	for dirty := range c.dirties {
		if dirty == nil {
			break
		}
		dRecord := c.empty.FromListHead(dirty).(Record)
		c.setLazy(dRecord)
	}
	atomic.AddInt32(&c.cntOfDirtyHandler, -1)

}

func (c *Cache) Set(v Record) error {

	//return c.setLazy(v)

	c.dirties <- v.PtrListHead()
	if atomic.LoadInt32(&c.cntOfDirtyHandler) < MaxOfDirtHandler {
		go c.handleDirties()
	}

	return nil

}

func (c *Cache) SetFn(updateFn func(*list_head.ListHead) Record) error {

	nr := updateFn(c.empty.PtrListHead())
	k := nr.CacheKey()
	c.RLock()
	nHead, found := c.key2ListHead[k]
	c.RUnlock()
	if !found {
		nHead = c.GetByPool()
	} else {
		c.startMu.Lock()
		nHead.Delete()
		c.startMu.Unlock()
	}
	updateFn(nHead)
	nRec := c.empty.FromListHead(nHead).(Record)
	c.Set(nRec)
	return nil

}

func (c *Cache) setLazy(v Record) error {

	vhead := v.PtrListHead()
	c.startMu.Lock()
	vhead.Init()
	c.startMu.Unlock()

	k := v.CacheKey()

	c.RLock()
	hit, found := c.key2ListHead[k]
	c.RUnlock()

	defer func() {
		if c.cnt >= c.max {
			if c.last == nil {
				c.startMu.Lock()
				c.last = c.start.Back()
				c.startMu.Unlock()
			}

			c.startMu.RLock()
			last := c.last
			c.last = c.last.Prev()
			c.startMu.RUnlock()

			c.startMu.Lock()
			last.Delete()
			c.startMu.Unlock()

			if rec, found := c.empty.FromListHead(last).(Record); found {
				c.Lock()
				k := rec.CacheKey()
				delete(c.key2ListHead, k)
				c.Unlock()
			}
			c.PushToPool(last)
			c.cnt--
		}
	}()

	if found && hit == v.PtrListHead() {
		goto AFTER
	}
	if found && hit != nil {
		c.startMu.Lock()
		if hit != hit.Prev() || hit != hit.Next() {
			hit.Delete()
		}
		c.startMu.Unlock()

		c.PushToPool(hit)
	}
	hit = vhead

	c.Lock()
	c.key2ListHead[k] = hit
	c.Unlock()

	c.startMu.Lock()
	if c.start == nil {
		c.start = hit
		c.startIsFront()
		c.startMu.Unlock()

		return nil
	}

	c.startIsFront()
	c.startMu.Unlock()

AFTER:

	if c.cType == TypeLFU {
		c.startMu.Lock()
		hit.Init()
		if c.last == nil {
			c.last = c.start.Back()
		}
		c.last.Add(hit)
		c.last = hit
		c.start = hit

		c.startIsFront()
		c.startMu.Unlock()

	} else {
		c.startMu.Lock()
		hit.Init()
		if c.start != nil && !c.start.IsFirst() {
			c.start = c.start.Front()
			c.startIsFront()
		}
		hit.Add(c.start.Front())
		if !hit.IsFirst() || hit.Front() != hit {
			fmt.Printf("not first?")
		}
		c.start = hit.Front()
		c.startIsFront()
		c.startMu.Unlock()

	}
	c.cnt++

	return nil

}

func (c *Cache) setFnLazy(v Record, updateFn func(*list_head.ListHead) Record) error {

	vhead := v.PtrListHead()
	vhead.Init()

	var k string
	if updateFn == nil {
		k = v.CacheKey()
	} else {
		nr := updateFn(c.empty.PtrListHead())
		k = nr.CacheKey()
	}

	defer func() {
		if c.cnt >= c.max {
			c.startMu.Lock()
			if c.last == nil {
				c.last = c.start.Back()
			}

			last := c.last
			c.last = c.last.Prev()

			last.Delete()
			c.startMu.Unlock()
			if rec, found := c.empty.FromListHead(last).(Record); found {
				c.Lock()
				delete(c.key2ListHead, rec.CacheKey())
				c.Unlock()
			}
			c.PushToPool(last)
			c.cnt--
		}

	}()

	if c.start != nil {
		goto ADD
	}
	if updateFn != nil {
		neoHead := c.GetByPool()

		c.startMu.Lock()
		neoHead.Init()
		c.startMu.Unlock()

		updateFn(neoHead)
		c.Lock()
		c.start = neoHead
		c.key2ListHead[k] = neoHead

		c.startIsFront()
		c.Unlock()

		c.cnt++
		return nil

	}
	c.startMu.Lock()
	vhead.Init()
	c.startMu.Unlock()

	c.Lock()

	c.start = vhead
	c.key2ListHead[k] = vhead

	c.startIsFront()
	c.Unlock()

	c.cnt++
	return nil

ADD:

	c.RLock()
	hit, found := c.key2ListHead[k]
	c.RUnlock()

	if updateFn == nil {
		goto ADD_NO_REUSE
	}

	if found {
		c.startMu.Lock()
		hit.Delete()
		c.startMu.Unlock()
		updateFn(hit)
	} else {
		hit = c.GetByPool()
		updateFn(hit)
		c.cnt++
	}

	c.Lock()
	c.key2ListHead[k] = hit
	c.Unlock()

	c.startMu.Lock()
	if c.cType == TypeLFU {

		hit.Init()
		if c.last != nil {
			c.last = c.start.Back()
		}
		c.last.Add(hit)
		c.start = hit
		c.last = c.last.Back()

	} else {
		hit.Init()
		hit.Add(c.start.Front())
		c.start = hit
	}

	c.startIsFront()
	c.startMu.Unlock()

	return nil

ADD_NO_REUSE:

	c.startMu.Lock()
	vhead.Init()
	c.startMu.Unlock()

	c.Lock()
	c.key2ListHead[k] = vhead
	c.Unlock()

	if found {
		c.startMu.Lock()
		if hit == c.start {
			c.start = hit.Delete()
		} else {
			hit.Delete()
		}

		c.startIsFront()
		c.startMu.Unlock()

		c.PushToPool(hit)
	} else {
		c.cnt++
	}
	c.startMu.Lock()
	if c.cType == TypeLFU {
		vhead.Add(c.start.Back())
		c.start.Back().Add(vhead)
		c.start = vhead
	} else {
		vhead.Add(c.start.Front())
		c.start = vhead
	}

	c.startIsFront()
	c.startMu.Unlock()

	return nil

}

type Fetcher func(head *list_head.ListHead)

func (c *Cache) Fetch(k string, fn Fetcher) error {

	c.RLock()
	hit, found := c.key2ListHead[k]
	c.RUnlock()

	if !found {
		return ErrorNotFound
	}

	if c.cType != TypeLRU && c.cType != TypeLFU {
		fn(hit)
		return nil
	}
	fn(hit)

	go c.PushReqOfUpdateList(k, hit, found)

	return nil
}

func (c *Cache) DataFromListead(head *list_head.ListHead) (r Record) {
	data := c.empty.FromListHead(head)
	succ := false
	if data == nil {
		goto FAIL
	}
	r, succ = data.(Record)
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
	hit, found = c.key2ListHead[k]
	c.RUnlock()
	if !found {
		return nil
	}
	return nil

}

func (c *Cache) PushReqOfUpdateList(k string, hit *list_head.ListHead, found bool) {
	//c.updateList(k, hit, found)

	c.pushReqOfUpdateListCh(k, hit, found)
}

func (c *Cache) pushReqOfUpdateListCh(k string, hit *list_head.ListHead, found bool) {

	req := c.reqPool.Get().(*reqfUpdateList)
	req.k = k
	req.hit = hit
	req.found = found
	c.reqUList <- req
	go c.updateListLazy()

}

func (c *Cache) Get(k string) (r Record, e error) {

	c.RLock()
	hit, found := c.key2ListHead[k]
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

	c.startMu.Lock()
	if found {
		hit.Delete()
	}
	hit.Add(c.start.Front())
	c.start = hit
	if !c.start.IsFirst() {
		c.start = c.start.Front()
	}

	c.startIsFront()
	c.startMu.Unlock()

	if !found {
		c.Lock()
		c.key2ListHead[k] = hit
		c.Unlock()
	}

}

func (c *Cache) updateListLazy() {
	if atomic.LoadInt32(&c.cntOfReqHandler) >= MaxOfDirtHandler {
		return
	}

	atomic.AddInt32(&c.cntOfReqHandler, 1)

	for req := range c.reqUList {
		if req == nil {
			break
		}
		if req.found {
			req.hit.Delete()
		}
		req.hit.Add(c.start.Front())
		c.start = req.hit
		c.startIsFront()
		if !req.found {
			c.Lock()
			c.key2ListHead[req.k] = req.hit
			c.Unlock()
		}
		c.reqPool.Put(req)
	}

	atomic.AddInt32(&c.cntOfReqHandler, -1)
	return
}
