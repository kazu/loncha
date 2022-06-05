package ecache

import (
	"sync"
	"sync/atomic"
	"unsafe"

	list_head "github.com/kazu/lista_encabezado"
)

var EmptyCacheHead *CacheHead = &CacheHead{}

type CacheHead struct {
	reference  int32
	registered bool
	list_head.ListHead
}

func (c *CacheHead) isRegister() bool {
	return c.registered
}

func (c *CacheHead) regist(t bool) {
	c.registered = t
}

func (c *CacheHead) cntOfRef() int {
	return int(c.reference)
}

func (c *CacheHead) referenced() {
	atomic.AddInt32(&c.reference, 1)
}

func (c *CacheHead) PtrListHead() *list_head.ListHead {
	return &(c.ListHead)
}

func (r *CacheHead) Offset() uintptr {
	return unsafe.Offsetof(r.ListHead)
}

func (c *CacheHead) fromListHead(l *list_head.ListHead) *CacheHead {
	return (*CacheHead)(list_head.ElementOf(EmptyCacheHead, l))
}

func (c *CacheHead) FromListHead(l *list_head.ListHead) list_head.List {
	return c.fromListHead(l)
}

func (c *CacheHead) used() bool {

	if c.isRegister() {
		return true
	}
	if c.cntOfRef() > 0 {
		return true
	}
	return false
}

type CacheEntry interface {
	Record
	isRegister() bool
	cntOfRef() int
	PtrCacheHead() *CacheHead
}

type CachePool interface {
	get() *CacheHead
	put(*CacheHead)
}

type syncPool sync.Pool

func (sp *syncPool) get() *CacheHead {
	return (*sync.Pool)(sp).Get().(*CacheHead)
}

func (sp *syncPool) put(d *CacheHead) {
	(*sync.Pool)(sp).Put(d)
}

type listPool struct {
	c      *Cache
	unused *list_head.ListHead
}

func (cp *listPool) get() (chead *CacheHead) {

	defer func() {
		//chead.regist(true)
		chead.referenced()
	}()

	if cp.unused != nil {

		for cur := cp.unused; !cur.Empty(); cur = cur.Next(list_head.WaitNoM()) {
			chead = EmptyCacheHead.fromListHead(cur)
			if !chead.used() {
				cp.unused = chead.Next(list_head.WaitNoM())
				return
			}
		}
		cp.unused = nil

	}

	for cur, prev := cp.c.last.Prev(list_head.WaitNoM()), cp.c.last.Prev(list_head.WaitNoM()).Prev(list_head.WaitNoM()); !cur.Empty(); cur, prev = prev, prev.Prev(list_head.WaitNoM()) {
		chead = EmptyCacheHead.fromListHead(cur)
		phead := EmptyCacheHead.fromListHead(prev)

		if chead.used() && phead.used() {
			goto ALLOC
		}

		if !chead.used() && phead.used() {
			cp.unused = chead.Next(list_head.WaitNoM())
			return
		}

	}
	if cp.c.start.Prev(list_head.WaitNoM()).Empty() && cp.c.start.Next(list_head.WaitNoM()).Empty() {
		goto ALLOC
	}

	chead = EmptyCacheHead.fromListHead(cp.c.start.Front().Next(list_head.WaitNoM()))
	cp.unused = chead.Next(list_head.WaitNoM())
	return chead

ALLOC:
	chead = cp.c.allocFn().PtrCacheHead()
	chead.Init()
	chead.referenced()
	//chead.regist(true)
	head := &chead.ListHead
	cp.c.last = cp.c.addLast(head)

	return chead
}

func (cp *listPool) put(chead *CacheHead) {
	chead.expire()
	cp.c.DeleteAndCheck(&chead.ListHead)
	hoge := "free"
	if cp.c.max*2 > cp.c.cnt+cp.c.cntOfunused {
		cp.c.last = cp.c.addLast(&chead.ListHead)
	} else {
		_ = hoge
		cp.c.cntOfunused--
	}
}

func (chead *CacheHead) expire() {
	chead.reference = 0
	chead.registered = false
}
