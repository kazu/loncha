// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"sync/atomic"
)

type MapString struct {
	baseMap
	dirty map[string]atomic.Value
}

type readMapStr struct {
	m       map[string]atomic.Value
	amended bool // include dirty map
}

func (r readMapStr) store(k string, v *ListHead) (ok bool) {
	ov, ok := r.m[k]
	if !ok {
		return ok
	}
	ohead := ov.Load().(*ListHead)
	return ov.CompareAndSwap(ohead, v)
}

func (m *MapString) Set(k string, v *ListHead) bool {

	read, succ := m.read.Load().(readMapStr)
	if !succ {
		m.read.Store(readMapStr{})
	}

	ensure := func(v *ListHead) {
		for _, fn := range m.onNewStores {
			fn(v)
		}
	}
	if ok := read.store(k, v); ok {
		defer ensure(v)
		return true
	}

	m.Lock()
	defer m.Unlock()
	_, ok := read.m[k]

	if !ok {
		_, ok = m.getDirty(k)
	}
	var av atomic.Value
	if ok {
		av = m.dirty[k]
	} else if !read.amended {
		m.read.Store(readMapStr{m: read.m, amended: true})
	}

	av.Store(v)
	if m.dirty == nil {
		m.dirty = map[string]atomic.Value{}
	}
	m.dirty[k] = av

	if !ok {
		defer ensure(v)
	}

	return true

}

func (m *MapString) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}
	m.read.Store(readMapStr{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

func (m *MapString) Get(k string) (v *ListHead, ok bool) {

	read := m.read.Load().(readMapStr)
	av, ok := read.m[k]
	if ok {
		v, ok = av.Load().(*ListHead)
	}
	if !ok && read.amended {
		m.Lock()
		defer m.Unlock()
		read := m.read.Load().(readMapStr)
		av, ok = read.m[k]
		if !ok && read.amended {
			v, ok = m.getDirty(k)
			m.missLocked()
		}
	}

	if !ok || v == nil {
		return nil, false
	}
	return
}

func (m *MapString) getDirty(k string) (v *ListHead, ok bool) {

	av, ok := m.dirty[k]
	if ok {
		v = av.Load().(*ListHead)
	}
	return
}

func (m *MapString) Delete(k string) bool {

	read, _ := m.read.Load().(readMapStr)
	av, ok := read.m[k]
	if !ok && read.amended {
		m.Lock()
		defer m.Unlock()
		read, _ := m.read.Load().(readMapStr)
		if !ok && read.amended {
			av, ok = m.dirty[k]
			delete(m.dirty, k)
			m.missLocked()
		}
	}

	if ok {
		ohead := av.Load().(*ListHead)
		return av.CompareAndSwap(ohead, nil)

	}
	return false
}

func (m *MapString) Len() int {

	return len(m.dirty)
}

func (m *MapString) Range(f func(key string, value *ListHead) bool) {

	read, _ := m.read.Load().(readMapStr)
	if read.amended {
		m.Lock()
		read, _ := m.read.Load().(readMapStr)
		if read.amended {
			read = readMapStr{m: m.dirty}
			m.read.Store(read)
			m.dirty = nil
			m.misses = 0
		}
		m.Unlock()
	}

	for k, e := range read.m {
		v, ok := e.Load().(*ListHead)
		if !ok {
			continue
		}

		if !f(k, v) {
			break
		}
	}

}
