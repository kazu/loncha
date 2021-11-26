// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list

package list_head

import (
	"sync/atomic"
	"unsafe"
)

type SkipHead struct {
	len   int64
	up    *SkipHead
	start *ListHead
	last  *ListHead
	ListHead
}

type SuperSkipHead struct {
	up    *SuperSkipHead
	start *SkipHead
	last  *SkipHead
}

func skipHeadFromListHead(head *ListHead) *SkipHead {
	return (*SkipHead)(ElementOf(&SkipHead{}, head))
}

func (e *SkipHead) Offset() uintptr {
	return unsafe.Offsetof(e.ListHead)
}

func (e *SkipHead) PtrListHead() *ListHead {
	return &e.ListHead
}

func (e *SkipHead) FromListHead(head *ListHead) List {
	return entryRmapFromListHead(head)
}

func (e *SkipHead) reduce(ocnt int) {
	if e.ListHead.Next().Empty() {
		return
	}

	n := skipHeadFromListHead(e.ListHead.Next())

	cnt := ocnt
	if cnt >= int(e.len) {
		cnt = int(e.len) - 1
	}

	for i := 0; i < cnt; i++ {
		e.last = e.last.Prev(WaitNoM())
	}
	atomic.AddInt64(&e.len, -int64(cnt))
	n.start = e.last.Next(WaitNoM())
	atomic.AddInt64(&n.len, int64(cnt))
}
