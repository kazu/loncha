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

	"github.com/cespare/xxhash"
)

type MapInf interface {
	Set(k string, v *ListHead) bool
	Get(k string) (v *ListHead, ok bool)
	Delete(k string) bool
	Len() int
	Range(f func(key string, value *ListHead) bool)
}

// MapGetSet ... usage for benchmark.
type MapGetSet interface {
	Set(k string, v *ListHead) bool
	Get(k string) (v *ListHead, ok bool)
}

type baseMap struct {
	sync.RWMutex
	read        atomic.Value
	onNewStores []func(*ListHead)

	misses int
}

type Map struct {
	baseMap
	dirty map[uint64]atomic.Value
}

type readMap struct {
	m       map[uint64]atomic.Value
	amended bool // include dirty map
}

func (r readMap) store(k, conflict uint64, kstr string, v *ListHead) (ok bool) {
	ov, ok := r.m[k]
	if !ok {
		return ok
	}
	ohead := ov.Load().(*entry)
	return ov.CompareAndSwap(ohead, &entry{key: kstr, v: v, conflict: conflict})
}

func (m *baseMap) onNewStore(fns ...func(*ListHead)) {
	m.onNewStores = fns
}

func (m *Map) Set(key string, v *ListHead) bool {
	k, conflict := KeyToHash(key)
	return m.Set2(k, conflict, key, v)
}

func (m *Map) Set2(k, conflict uint64, kstr string, v *ListHead) bool {

	read, succ := m.read.Load().(readMap)
	if !succ {
		m.read.Store(readMap{})
	}

	ensure := func(v *ListHead) {
		for _, fn := range m.onNewStores {
			fn(v)
		}
	}
	if ok := read.store(k, conflict, kstr, v); ok {
		defer ensure(v)
		return true
	}

	m.Lock()
	defer m.Unlock()
	_, ok := read.m[k]

	if !ok {
		_, ok = m.getDirty(k, conflict)
	}
	var av atomic.Value
	if ok {
		av = m.dirty[k]
	} else if !read.amended {
		m.read.Store(readMap{m: read.m, amended: true})
	}

	av.Store(&entry{v: v, conflict: conflict})
	if m.dirty == nil {
		m.dirty = map[uint64]atomic.Value{}
	}
	m.dirty[k] = av

	if !ok {
		defer ensure(v)
	}

	return true
}

func (m *Map) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}
	m.read.Store(readMap{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

func (m *Map) Get(key string) (v *ListHead, ok bool) {

	return m.Get2(KeyToHash(key))
}

func (m *Map) Get2(k, conflict uint64) (v *ListHead, ok bool) {

	read := m.read.Load().(readMap)
	av, ok := read.m[k]
	var e *entry
	if ok {
		e, ok = av.Load().(*entry)
		if e.conflict != conflict {
			ok = false
		} else {
			v = e.v
		}
	}
	if !ok && read.amended {
		m.Lock()
		defer m.Unlock()
		read := m.read.Load().(readMap)
		av, ok = read.m[k]
		if !ok && read.amended {
			v, ok = m.getDirty(k, conflict)
			m.missLocked()
		}
	}

	if !ok || v == nil {
		return nil, false
	}
	return
}

func (m *Map) getDirty(k, conflict uint64) (v *ListHead, ok bool) {
	// m.RLock()
	// defer m.RUnlock()

	av, ok := m.dirty[k]
	var e *entry
	if ok {
		e = av.Load().(*entry)
		if e.conflict == conflict {
			v = e.v
		}
	}
	return
}

func (m *Map) Delete(key string) bool {
	k, _ := KeyToHash(key)

	read, _ := m.read.Load().(readMap)
	av, ok := read.m[k]
	if !ok && read.amended {
		m.Lock()
		defer m.Unlock()
		read, _ := m.read.Load().(readMap)
		if !ok && read.amended {
			av, ok = m.dirty[k]
			delete(m.dirty, k)
			m.missLocked()
		}
	}

	if ok {
		ohead := av.Load().(*entry)
		//ohead.conflict = conflict
		return av.CompareAndSwap(ohead, nil)

	}
	return false
}

func (m *Map) Len() int {

	return len(m.dirty)
}

func (m *Map) Range(f func(key string, value *ListHead) bool) {

	read, _ := m.read.Load().(readMap)
	if read.amended {
		m.Lock()
		read, _ := m.read.Load().(readMap)
		if read.amended {
			read = readMap{m: m.dirty}
			m.read.Store(read)
			m.dirty = nil
			m.misses = 0
		}
		m.Unlock()
	}

	for _, e := range read.m {
		v, ok := e.Load().(*entry)
		if !ok {
			continue
		}

		if !f(v.key, v.v) {
			break
		}
	}

}

type MapWithLock struct {
	sync.RWMutex
	m map[string]*ListHead
}

func (m *MapWithLock) Set(k string, v *ListHead) bool {
	m.Lock()
	defer m.Unlock()

	if m.m == nil {
		m.m = map[string]*ListHead{}
	}

	m.m[k] = v
	return true

}

func (m *MapWithLock) Get(k string) (v *ListHead, ok bool) {

	m.RLock()
	defer m.RUnlock()

	v, ok = m.m[k]
	return

}

func (m *MapWithLock) Delete(k string) bool {

	m.Lock()
	defer m.Unlock()
	delete(m.m, k)
	return true
}

func (m *MapWithLock) Len() int { return len(m.m) }

func (m *MapWithLock) Range(f func(key string, value *ListHead) bool) {

	for k, v := range m.m {

		if !f(k, v) {
			break
		}

	}

}

type entry struct {
	key      string
	conflict uint64
	v        *ListHead
}

const cntOfShard = 32

type ShardMap struct {
	shards [cntOfShard]MapGetSet
}

func (shard *ShardMap) InitByFn(fn func(int) MapGetSet) {

	for i := range shard.shards {
		shard.shards[i] = fn(i)
	}
}

func (shard *ShardMap) Get(k string) (v *ListHead, ok bool) {

	key, conflict := shard.KeyToHash(k)
	i := key % cntOfShard
	if m, isMap := shard.shards[i].(*Map); isMap {
		return m.Get2(key, conflict)
	}

	return shard.shards[i].Get(k)

}

func (shard *ShardMap) Set(k string, v *ListHead) bool {

	key, conflict := shard.KeyToHash(k)
	i := key % cntOfShard
	if m, isMap := shard.shards[i].(*Map); isMap {
		return m.Set2(key, conflict, k, v)
	}

	return shard.shards[i].Set(k, v)
}

type stringStruct struct {
	str unsafe.Pointer
	len int
}

//go:noescape
//go:linkname memhash runtime.memhash
func memhash(p unsafe.Pointer, h, s uintptr) uintptr

func MemHash(data []byte) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&data))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

func MemHashString(str string) uint64 {
	ss := (*stringStruct)(unsafe.Pointer(&str))
	return uint64(memhash(ss.str, 0, uintptr(ss.len)))
}

func (shard *ShardMap) KeyToHash(key interface{}) (uint64, uint64) {
	return KeyToHash(key)
}

func KeyToHash(key interface{}) (uint64, uint64) {

	if key == nil {
		return 0, 0
	}
	switch k := key.(type) {
	case uint64:
		return k, 0
	case string:
		return MemHashString(k), xxhash.Sum64String(k)
	case []byte:
		return MemHash(k), xxhash.Sum64(k)
	case byte:
		return uint64(k), 0
	case int:
		return uint64(k), 0
	case int32:
		return uint64(k), 0
	case uint32:
		return uint64(k), 0
	case int64:
		return uint64(k), 0
	default:
		panic("Key type not supported")
	}
}
