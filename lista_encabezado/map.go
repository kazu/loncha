// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import "sync"

type Map struct {
	sync.RWMutex
	data        map[string]*ListHead
	onNewStores []func(*ListHead)
}

func (m *Map) onNewStore(fns ...func(*ListHead)) {
	m.onNewStores = fns

}

func (m *Map) Set(k string, v *ListHead) bool {
	m.Lock()
	defer m.Unlock()
	_, ok := m.Get(k)
	m.data[k] = v
	if ok {
		for _, fn := range m.onNewStores {
			fn(v)
		}
	}

	return true
}

func (m *Map) Get(k string) (v *ListHead, ok bool) {
	m.RLock()
	defer m.RUnlock()

	v, ok = m.data[k]
	return
}

func (m *Map) Delete(k string) (ok bool) {
	m.Lock()
	defer m.Unlock()

	delete(m.data, k)
	return true
}

func (m *Map) Len() int {

	return len(m.data)
}
