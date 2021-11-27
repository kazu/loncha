// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"sync/atomic"
)

type RMap2 struct {
	baseMap
	limit int
	dirty *HMap
}

func NewRMap2() *RMap2 {

	r := &RMap2{
		limit: 1000,
		//dirty: newHMap(),
	}
	hmap := NewHMap()
	r.dirty = hmap
	return r
}

func (m *RMap2) Set(key string, v *ListHead) bool {
	k, conflict := KeyToHash(key)
	return m.Set2(k, conflict, key, v)
}

func (m *RMap2) getDirtyEntry(k, conflict uint64) (e *entryRmap) {

	he, _ := m.dirty._get(k, conflict)
	if he == nil {
		return
	}

	e, _ = he.Value().(*entryRmap)
	// m.traverseDirty(nil, keySearch2(k), func(dst, node *ListHead) {
	// 	if dst != nil {
	// 		e = entryRmapFromListHead(dst)
	// 	}
	// })

	if e == nil {
		return
	}
	if e.k != k || e.conflict != conflict {
		e = nil
	}

	return

}

func (m *RMap2) getDirty(k, conflict uint64) (v *ListHead, ok bool) {

	e := m.getDirtyEntry(k, conflict)
	if e == nil {
		return nil, false
	}
	return e.v, true
}

func (m *RMap2) Set2(k, conflict uint64, kstr string, v *ListHead) bool {
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

	if !ok {
		e := &entryRmap{
			key:      kstr,
			k:        k,
			conflict: conflict,
			v:        v,
		}
		e.Init()

		m.dirty._set(k, conflict, kstr, e)

		if len(read.m) == 0 {
			m.storeReadFromDirty(true)
		}
		defer ensure(v)
	}
	return true
}

// func (m *RMap2) ValidateDirty() {

// 	cnt := 0
// 	for cur := m.dirty.list.start.Prev().Next(); !cur.Empty(); cur = cur.Next() {
// 		next := cur.Next()
// 		if next.Empty() {
// 			break
// 		}
// 		cEntry := entryRmapFromListHead(cur)
// 		nEntry := entryRmapFromListHead(next)
// 		if cEntry.k > nEntry.k {
// 			_ = "invalid order"
// 		}
// 		cnt++
// 	}

// }

func (m *RMap2) isNotRestoreRead() bool {

	read := m.read.Load().(*readMap)
	return m.misses <= len(read.m) && m.misses < int(m.dirty.len)
	//return m.misses < int(m.dirty.len)
}

func (m *RMap2) missLocked() {
	m.misses++
	if m.isNotRestoreRead() {
		return
	}
	m.storeReadFromDirty(false)
	h := NewHMap()
	m.dirty = h
	m.misses = 0
}

func (m *RMap2) Get(key string) (v *ListHead, ok bool) {

	return m.Get2(KeyToHash(key))
}

func (m *RMap2) Get2(k, conflict uint64) (v *ListHead, ok bool) {

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

func (m *RMap2) Delete(key string) bool {
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
func (m *RMap2) Len() int {

	return int(m.dirty.len)
}

func (m *RMap2) storeReadFromDirty(amended bool) {

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
		// m.dirty.each(func(v interface{}) {
		// 	entry, _ := v.(*entryRmap)
		// 	a := atomic.Value{}
		// 	a.Store(entry)
		// 	nread.m[entry.k] = a
		// })
		m.dirty.eachEntry(m.dirty.start.Prev().Next(), func(e *entryHMap) {
			entry, _ := e.value.(*entryRmap)
			a := atomic.Value{}
			a.Store(entry)
			nread.m[entry.k] = a
		})

		if len(nread.m) == 0 {
			break
		}
		if len(oread.m) == 0 {
			nread.amended = true
		}
		if m.read.CompareAndSwap(oread, nread) {
			m.Unlock()
			h := NewHMap()
			m.dirty = h
			m.Lock()
			break
		}
		_ = "???"
	}
}
