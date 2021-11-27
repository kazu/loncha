// Copyright 2019-2201 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"unsafe"
)

type SampleItem struct {
	K interface{}
	V interface{}
	MapHead
}

var sampleItem MapItem = &SampleItem{}

var EmptySampleHMapEntry SampleItem = SampleItem{}

func SampleItemFromListHead(head *ListHead) *SampleItem {
	return (*SampleItem)(ElementOf(&EmptySampleHMapEntry, head))
}

func (s *SampleItem) Offset() uintptr {
	return unsafe.Offsetof(s.ListHead)
}

func (s *SampleItem) PtrMapeHead() *MapHead {
	return &(s.MapHead)
}

func (s *SampleItem) hmapEntryFromListHead(lhead *ListHead) *SampleItem {
	return SampleItemFromListHead(lhead)
}

func (s *SampleItem) HmapEntryFromListHead(lhead *ListHead) HMapEntry {
	return s.hmapEntryFromListHead(lhead)
}

func (s *SampleItem) Key() interface{} {
	return s.K
}

func (s *SampleItem) Value() interface{} {
	return s.V
}

func (s *SampleItem) SetValue(v interface{}) bool {
	s.V = v
	return true
}

func (s *SampleItem) Next() HMapEntry {
	return s.hmapEntryFromListHead(s.next)
}
func (s *SampleItem) Prev() HMapEntry {
	return s.hmapEntryFromListHead(s.prev)
}

func (s *SampleItem) PtrMapHead() *MapHead {
	return &s.MapHead
}
