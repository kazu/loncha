// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type RMap struct {
	baseMap
	limit int
	dirty Dirty
}

func NewRMap() *RMap {

	return &RMap{
		limit: 1000,
	}
}

type Dirty struct {
	list *SkipHead
	len  int64
	slen int64
}

type entryRmap struct {
	key      string
	k        uint64
	conflict uint64
	v        *ListHead
	sync.RWMutex
	ListHead
}

func (e *entryRmap) Offset() uintptr {
	return unsafe.Offsetof(e.ListHead)
}

func (e *entryRmap) PtrListHead() *ListHead {
	return &e.ListHead
}

func (e *entryRmap) FromListHead(head *ListHead) List {
	return entryRmapFromListHead(head)
}

func entryRmapFromListHead(head *ListHead) *entryRmap {
	return (*entryRmap)(ElementOf(&entryRmap{}, head))
}

func MutexFn(head *ListHead) *sync.RWMutex {
	e := (*entryRmap)(ElementOf(&entryRmap{}, head))
	if e == nil {
		_ = "???"
	}
	return &e.RWMutex
}

func (m *RMap) initDirty() {
	m.Lock()
	defer m.Unlock()
	MODE_CONCURRENT = true

	m.dirty.list = &SkipHead{len: 0}
	m.dirty.list.Init()

	llist := &ListHead{}
	llist.InitAsEmpty()
	m.dirty.list.start = llist.Prev()
	m.dirty.list.last = llist.Next()

	m.dirty.len = 0
	m.dirty.slen = 1
}

func (m *RMap) Set(key string, v *ListHead) bool {
	k, conflict := KeyToHash(key)
	return m.Set2(k, conflict, key, v)
}

func keySearch(k uint64) func(e *entryRmap) bool {

	return func(e *entryRmap) bool {
		if e.k >= k {
			return true
		}
		return false
	}
}

func keySearch2(k uint64) func(head *ListHead) bool {

	return func(head *ListHead) bool {
		e := entryRmapFromListHead(head)
		return keySearch(k)(e)

	}
}

func (m *RMap) getDirtyEntry(k, conflict uint64) (e *entryRmap) {

	if m.dirty.list == nil {
		m.initDirty()
	}

	// e = m.bsearchDirty(func(e *entryRmap) bool {
	// 	if e.k >= k {
	// 		return true
	// 	}
	// 	return false
	// })
	m.traverseDirty(nil, keySearch2(k), func(dst, node *ListHead) {
		if dst != nil {
			e = entryRmapFromListHead(dst)
		}
	})

	if e == nil {
		return
	}
	if e.k != k || e.conflict != conflict {
		e = nil
	}

	return

}

func (m *RMap) getDirty(k, conflict uint64) (v *ListHead, ok bool) {

	e := m.getDirtyEntry(k, conflict)
	if e == nil {
		return nil, false
	}
	return e.v, true
}

func (m *RMap) Set2(k, conflict uint64, kstr string, v *ListHead) bool {
	read, succ := m.read.Load().(*readMap)
	if !succ {
		m.read.Store(&readMap{})
		read, succ = m.read.Load().(*readMap)
	}

	ensure := func(v *ListHead) {
		for _, fn := range m.onNewStores {
			fn(v)
		}
	}
	if ok := read.store2(k, conflict, kstr, v); ok {
		defer ensure(v)
		return true
	}

	_, ok := read.m[k]
	if !ok {
		e := m.getDirtyEntry(k, conflict)
		if e != nil {
			ok = true
			e.v = v
		}
	}
	if !ok && !read.amended {
		m.storeReadFromDirty(true)
	}
	if m.dirty.list == nil {
		m.initDirty()
	}

	if !ok {
		e := &entryRmap{
			key:      kstr,
			k:        k,
			conflict: conflict,
			v:        v,
		}
		e.Init()
		ostart := unsafe.Pointer(m.dirty.list.start)
		//	olast := unsafe.Pointer(m.dirty.list.

		m.insertDirty(e, keySearch(e.k))
		atomic.AddInt64(&m.dirty.len, 1)
		if ostart != unsafe.Pointer(m.dirty.list.start) {
			//		olast != unsafe.Pointer(m.dirty.list.last) {
			_ = "???"
		}
		if m.dirty.len > 1 && !e.Prev().Empty() &&
			entryRmapFromListHead(e.Prev()).k > e.k {
			pe := entryRmapFromListHead(e.Prev())
			_ = pe
			_ = "???"
		}

		if int(m.dirty.len) > m.limit*int(m.dirty.slen) {
			m.SplitDirty()
		}

		if len(read.m) == 0 {
			m.storeReadFromDirty(true)
		}
		defer ensure(v)
	}
	return true
}

func (m *RMap) ValidateDirty() {

	cnt := 0
	for cur := m.dirty.list.start.Prev().Next(); !cur.Empty(); cur = cur.Next() {
		next := cur.Next()
		if next.Empty() {
			break
		}
		cEntry := entryRmapFromListHead(cur)
		nEntry := entryRmapFromListHead(next)
		if cEntry.k > nEntry.k {
			_ = "invalid order"
		}
		cnt++
	}

}

func (m *RMap) SplitDirty() {

	slastEmpty := skipHeadFromListHead(m.dirty.list.Back().Next(WaitNoM()))
	slast := skipHeadFromListHead(slastEmpty.Prev(WaitNoM()))
	nSlist := &SkipHead{len: 0}
	nSlist.Init()

	nSlist.last = slast.last

	slastEmpty.insertBefore(&nSlist.ListHead)
	m.dirty.slen++

	pShead := skipHeadFromListHead(nSlist.Prev())
	pShead.reduce(int(pShead.len) / 2)
}

func (m *RMap) _SplitDirty() {

	//m.ValidateDirty()

	slastEmpty := skipHeadFromListHead(m.dirty.list.Back().Next(WaitNoM()))
	slast := skipHeadFromListHead(slastEmpty.Prev(WaitNoM()))
	nSlist := &SkipHead{}
	nSlist.Init()

	nSlist.last = slast.last

	slastEmpty.insertBefore(&nSlist.ListHead)
	m.dirty.slen++

	sub := m.limit*int(m.dirty.slen) - int(m.dirty.len)
	cntOfMove := sub / int(m.dirty.slen)

	moved := cntOfMove
	for scur, snext := m.dirty.list.Prev(WaitNoM()).Next(WaitNoM()), m.dirty.list.Prev(WaitNoM()).Next(WaitNoM()).Next(WaitNoM()); !snext.Empty(); scur, snext = snext, snext.Next(WaitNoM()) {
		sheadCur := skipHeadFromListHead(scur)
		sheadNext := skipHeadFromListHead(snext)

		oMoved := moved
		for cur := sheadCur.last.Prev(WaitNoM()); !cur.Empty(); cur = cur.Prev(Lock(MutexFn)) {
			moved--
			if moved <= 1 {
				sheadCur.last = cur.Prev(Lock(MutexFn))
				sheadNext.start = cur
				break
			}
		}
		moved = 0
		moved += oMoved + cntOfMove
	}

}

func keyRangeFromSkipListHead(scur *ListHead) (uint64, uint64) {
	shead := skipHeadFromListHead(scur)
	var sEntry, lEntry *entryRmap
	if !shead.start.Empty() {
		sEntry = entryRmapFromListHead(shead.start)
	} else {
		sEntry = entryRmapFromListHead(shead.start.Next(WaitNoM()))
	}
	if !shead.last.Empty() {
		lEntry = entryRmapFromListHead(shead.last)
	} else {
		lEntry = entryRmapFromListHead(shead.last.Prev(WaitNoM()))
	}
	return sEntry.k, lEntry.k

}

func (m *RMap) insertDirty(e *entryRmap, cond func(*entryRmap) bool) {

	m._insertDirty(e, func(head *ListHead) bool {
		ent := entryRmapFromListHead(head)
		return cond(ent)
	})
	for scur := m.dirty.list.Prev(WaitNoM()).Next(WaitNoM()); !scur.Empty(); scur = scur.Next(Lock(MutexFn)) {
		_, lastK := keyRangeFromSkipListHead(scur)
		if e.k <= lastK {
			shead := skipHeadFromListHead(scur)
			atomic.AddInt64(&shead.len, 1)
			if !shead.Next().Empty() && shead.len > int64(m.limit) {
				shead.reduce(int(shead.len) / 10)
			}

			break
		}
	}
}

func (m *RMap) _insertDirty(e *entryRmap, cond func(*ListHead) bool) {

	//defer m.ValidateDirty()

	insert := func(dst, node *ListHead) {
		if !dst.Empty() && entryRmapFromListHead(dst).k < entryRmapFromListHead(node).k {
			dstk := entryRmapFromListHead(dst).k
			nodek := entryRmapFromListHead(node).k
			_, _ = dstk, nodek
			if !dst.next.Empty() {
				nk := entryRmapFromListHead(dst.next).k
				_ = nk
			}
			dst.Next(WaitNoM()).insertBefore(node)
			return

		}
		dst.insertBefore(node)
	}
	m.traverseDirty(e, cond, insert)

}

func (m *RMap) traverseDirty(e *entryRmap, cond func(*ListHead) bool, success func(dst, node *ListHead)) {

	onSuccess := func(dst *ListHead, node *entryRmap) {
		if e != nil {
			success(dst, &node.ListHead)
		} else {
			success(dst, nil)
		}
	}

	if m.dirty.len == 0 {
		onSuccess(m.dirty.list.last, e)

		return
	}
	slist := m.dirty.list.Prev(WaitNoM()).Next(WaitNoM())

	var shead *SkipHead

	for scur := slist; !scur.Empty(); scur = scur.Next(WaitNoM()) {
		shead = skipHeadFromListHead(scur)
		start := shead.start.Prev(WaitNoM()).Next(WaitNoM())
		last := shead.last.Next(WaitNoM()).Prev(WaitNoM())

		if cond(start) {
			onSuccess(start, e)
			return
		}
		if !cond(last) {
			continue
		}
		for cur := start; !cur.Empty(); cur = cur.Next(Lock(MutexFn)) {
			if cond(cur) {
				onSuccess(cur, e)
				return
			}
		}
		onSuccess(last, e)
		return
	}

	// for scur := slist; !scur.Empty(); scur = scur.Next(WaitNoM()) {
	// 	shead = skipHeadFromListHead(scur)

	// 	// if last.k < k -> snext
	// 	if !cond(shead.last.Next(WaitNoM()).Prev(WaitNoM())) {
	// 		if nlist := slist.Next(WaitNoM()); !nlist.Empty() {
	// 			continue
	// 		}
	// 		onSuccess(shead.last.Next(WaitNoM()).Prev(WaitNoM()), e)
	// 		return
	// 	}
	// 	// last.k >= k

	// 	// if start.k >= k . return start
	// 	if cond(shead.start.Prev(WaitNoM()).Next(WaitNoM())) {
	// 		onSuccess(shead.start.Prev(WaitNoM()).Next(WaitNoM()), e)
	// 		return
	// 	}
	// 	// start.k < k

	// 	// start.k < k  < last.k
	// 	for cur := shead.start.Next(WaitNoM()); !cur.Empty(); cur = cur.Next(Lock(MutexFn)) {
	// 		if cond(cur) {
	// 			onSuccess(cur.Next(Lock(MutexFn)), e)
	// 			return
	// 		}
	// 	}
	// 	if shead == nil {
	// 		_ = "???"
	// 	}
	// 	onSuccess(shead.last.Next(WaitNoM()).Prev(WaitNoM()), e)
	// 	return
	// }

	onSuccess(shead.last.Next(WaitNoM()).Prev(WaitNoM()), e)
}

// func (m *RMap) _traverseDirty(e *entryRmap, cond func(*ListHead) bool, success func(dst, node *ListHead)) {

// 	onSuccess := func(dst *ListHead, node *entryRmap) {
// 		if e != nil {
// 			success(dst, &node.ListHead)
// 		} else {
// 			success(dst, nil)
// 		}
// 	}

// 	if m.dirty.len == 0 {
// 		onSuccess(m.dirty.list.last, e)

// 		return
// 	}

// 	slist := skipHeadFromListHead(m.dirty.list.Prev(WaitNoM()).Next(WaitNoM()))
// 	// insert := func(dst, node *ListHead) {
// 	// 	dst.insertBefore(node)
// 	// }
// 	toPrev := false
// 	toNext := false
// 	for {

// 		if cond(slist.start.Next(WaitNoM())) && cond(slist.last.Prev(WaitNoM())) {
// 			if nlist := slist.Prev(WaitNoM()); !nlist.Empty() {
// 				if toNext {
// 					onSuccess(slist.start.Next(WaitNoM()), e)
// 					return
// 				}
// 				slist = skipHeadFromListHead(nlist)
// 				toPrev = true
// 				continue
// 			}
// 			onSuccess(slist.start.Next(WaitNoM()), e)
// 			break
// 		}
// 		if !cond(slist.start.Next(WaitNoM())) && !cond(slist.last.Prev(WaitNoM())) {
// 			if nlist := slist.Next(WaitNoM()); !nlist.Empty() {
// 				if toPrev {
// 					onSuccess(slist.last, e)
// 					return
// 				}
// 				slist = skipHeadFromListHead(nlist)
// 				toNext = true
// 				continue
// 			}
// 			onSuccess(slist.last.Prev(WaitNoM()), e)
// 			break
// 		}

// 		for cur := slist.start.Next(WaitNoM()); !cur.Empty(); cur = cur.Next(Lock(MutexFn)) {
// 			if cond(cur) {
// 				onSuccess(cur.Next(Lock(MutexFn)), e)
// 				return
// 			}
// 		}
// 	}
// }
func (m *RMap) isNotRestoreRead() bool {

	read := m.read.Load().(*readMap)
	return m.misses <= len(read.m) && m.misses < int(m.dirty.len)
	//return m.misses < int(m.dirty.len)

}

func (m *RMap) missLocked() {
	m.misses++
	if m.isNotRestoreRead() {
		return
	}
	m.storeReadFromDirty(false)
	m.initDirty()
	m.misses = 0
}

func (m *RMap) Get(key string) (v *ListHead, ok bool) {

	return m.Get2(KeyToHash(key))
}

func (m *RMap) Get2(k, conflict uint64) (v *ListHead, ok bool) {

	read := m.read.Load().(*readMap)
	av, ok := read.m[k]
	var e *entryRmap
	if ok {
		e, ok = av.Load().(*entryRmap)
		if e.conflict != conflict {
			ok = false
		} else {
			v = e.v
		}
	}

	if !ok && read.amended {
		// m.Lock()
		// defer m.Unlock()
		read := m.read.Load().(*readMap)
		av, ok = read.m[k]
		if !ok && read.amended {
			v, ok = m.getDirty(k, conflict)
			m.missLocked()
		}
	}
	return
}

func (m *RMap) Delete(key string) bool {
	k, conflict := KeyToHash(key)

	read, _ := m.read.Load().(*readMap)
	av, ok := read.m[k]
	if !ok && read.amended {
		m.Lock()
		defer m.Unlock()
		read, _ := m.read.Load().(*readMap)
		if !ok && read.amended {
			// av, ok = m.dirty[k]
			// delete(m.dirty, k)
			e := m.getDirtyEntry(k, conflict)
			if e != nil {
				e.MarkForDelete()
				atomic.AddInt64(&m.dirty.len, -1)
			}

			m.missLocked()
		}
	}

	if ok {
		ohead := av.Load().(*entryRmap)
		//ohead.conflict = conflict
		return av.CompareAndSwap(ohead, nil)

	}
	return false
}
func (m *RMap) Len() int {

	return int(m.dirty.len)
}

// func (m *RMap) dirtymiddle(oBegin, oEnd *ListHead) (middle *ListHead) {

// 	if m.dirty.start == nil {
// 		return nil
// 	}
// 	if m.dirty.len == 0 {
// 		return nil
// 	}

// 	begin := oBegin
// 	end := oEnd

// 	for {
// 		if begin == end || begin.Prev(WaitNoM()) == end {
// 			middle = end
// 			break
// 		}
// 		if begin.Empty() && end.Empty() {
// 			middle = nil
// 			break
// 		}
// 		if !begin.Empty() {
// 			begin = begin.Next(Lock(MutexFn))
// 		} else {
// 			begin = begin.Next()
// 		}
// 		if !end.Empty() {
// 			end = end.Prev(Lock(MutexFn))
// 		} else {
// 			end = end.Next()
// 		}

// 	}
// 	return
// }

// func (m *RMap) bsearchDirty(cond func(*entryRmap) bool) *entryRmap {

// 	head := m.bsearch(func(cur *ListHead) bool {
// 		e := entryRmapFromListHead(cur)
// 		return cond(e)
// 	})
// 	return entryRmapFromListHead(head)

// }

// func (m *RMap) bsearch(cond func(*ListHead) bool) *ListHead {

// 	begin := m.dirty.start.Prev().Next()
// 	end := m.dirty.last.Next().Prev()
// 	if m.dirty.middle == nil {
// 		m.dirty.middle = m.dirtymiddle(begin, end)
// 	}
// 	middle := m.dirty.middle

// 	for {

// 		//middle := m.dirtymiddle(begin, end)
// 		if middle == nil {
// 			return nil
// 		}
// 		// if middle == m.dirty.start || middle == m.dirty.last {
// 		// 	return nil
// 		// }

// 		if cond(begin) {
// 			return begin
// 		}
// 		if cond(middle) {
// 			end = middle
// 			middle = m.dirtymiddle(begin, end)
// 			if end == middle {
// 				return middle
// 			}
// 			continue
// 		}
// 		if !cond(end) {
// 			return end
// 		}
// 		if begin == middle {
// 			return end
// 		}
// 		begin = middle
// 		middle = m.dirtymiddle(begin, end)

// 	}

// }

func (m *RMap) storeReadFromDirty(amended bool) {

	m.Lock()
	defer m.Unlock()

	for {
		oread, _ := m.read.Load().(*readMap)

		nread := &readMap{
			m:       map[uint64]atomic.Value{},
			amended: amended,
		}

		//MENTION: not require copy oread ?
		for k, a := range oread.m {
			_, ok := a.Load().(*entryRmap)
			if !ok {
				continue
			}
			nread.m[k] = a
		}

		if m.dirty.list == nil {
			m.initDirty()
		}
		for cur := m.dirty.list.start.Next(Lock(MutexFn)); !cur.Empty(); cur = cur.Next(Lock(MutexFn)) {
			entry := entryRmapFromListHead(cur)
			a := atomic.Value{}
			a.Store(entry)
			nread.m[entry.k] = a
		}
		if len(nread.m) == 0 {
			break
		}
		if len(oread.m) == 0 {
			nread.amended = true
		}
		if m.read.CompareAndSwap(oread, nread) {
			m.Unlock()
			m.initDirty()
			m.Lock()
			break
		}
		_ = "???"
	}
}

func (r *readMap) store2(k, conflict uint64, kstr string, v *ListHead) (ok bool) {
	ov, ok := r.m[k]
	if !ok {
		return ok
	}
	ohead := ov.Load().(*entryRmap)
	return ov.CompareAndSwap(ohead, &entryRmap{key: kstr, v: v, conflict: conflict})
}
