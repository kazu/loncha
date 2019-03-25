// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lonacha/list_head is like a kernel's LIST_HEAD
// list_head is used by lonacha/gen/containers_list
package list_head

import (
	"errors"
	"sync/atomic"
	"unsafe"
)

var (
	MODE_CONCURRENT = false
)

type ListHead struct {
	prev *ListHead
	next *ListHead
}

func (head *ListHead) Init() {
	head.prev = head
	head.next = head
}

func (head *ListHead) Prev() *ListHead {
	return head.prev
}

func (head *ListHead) isDeleted() bool {
	ptr := uintptr(unsafe.Pointer(head.next))
	if ptr&3 > 0 {
		return true
	}
	return false
}
func (head *ListHead) Next() *ListHead {

	if head.isDeleted() {
		//FIXME: dosent work if mark > 3
		return (*ListHead)(unsafe.Pointer((uintptr(unsafe.Pointer(head.next)) ^ 3)))
	}
	return head.next
}

func listAdd(new, prev, next *ListHead) {
	if prev != next {
		next.prev, new.next, new.prev, prev.next = new, next, prev, new
	} else {
		prev.next, new.prev = new, prev
	}
}

func listAddWitCas(new, prev, next *ListHead) (err error) {

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(next),
		unsafe.Pointer(new)) {
		if prev != next {
			next.prev, new.next, new.prev = new, next, prev
		} else {
			new.prev = prev
		}
		return
	}
	return errors.New("cas conflict")

}

func (head *ListHead) Add(new *ListHead) {
	if MODE_CONCURRENT {
		for true {
			err := listAddWitCas(new, head, head.next)
			if err == nil {
				break
			}
		}
		return
	}
	listAdd(new, head, head.next)
}

func (l *ListHead) MarkForDelete() (err error) {

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&l.next)),
		unsafe.Pointer(l.next),
		unsafe.Pointer(uintptr(unsafe.Pointer(l.next))+1)) {
		return
	}
	return errors.New("cas conflict(fail mark)")
}

func (l *ListHead) DeleteWithCas() (err error) {

	if l.IsFirst() {
		l.next.prev = l.next
		return
	} else if l.IsLast() {
		err = l.MarkForDelete()
		if err != nil {
			return err
		}

		if atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(&l.prev.next)),
			unsafe.Pointer(l),
			unsafe.Pointer(l.prev)) {
			return
		}
		return errors.New("Delete fail retry")
	} else {
		err = l.MarkForDelete()
		if err != nil {
			return err
		}

		if atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(&l.prev.next)),
			unsafe.Pointer(l),
			unsafe.Pointer(l.next)) {

			l.next.prev = l.prev
			return
		}
		return errors.New("Delete fail retry")
	}

}

func (l *ListHead) Delete() *ListHead {

	if MODE_CONCURRENT {
		for true {
			err := l.DeleteWithCas()
			if err == nil {
				break
			}
		}
	} else {

		if l.IsFirst() {
			l.next.prev = l.next
		} else if l.IsLast() {
			l.prev.next = l.prev
		} else {
			l.next.prev, l.prev.next = l.prev, l.next
		}
	}
	l.next, l.prev = l, l

	return l.next

}

func (l *ListHead) Empty() bool {
	return l.next == l
}

func (l *ListHead) IsLast() bool {
	return l.next == l
}

func (l *ListHead) IsFirst() bool {
	return l.prev == l
}

func (l *ListHead) Len() (cnt int) {

	cnt = 1
	for back := l.Back(); back.prev != back; back = back.prev {
		cnt++
	}
	return cnt
}

func (l *ListHead) Front() (head *ListHead) {

	for head = l; head.prev != head; head = head.prev {
		if head.IsFirst() {
			return head
		}
	}
	//panic("front not found")
	return
}

func (l *ListHead) Back() (head *ListHead) {

	for head = l; head.next != head; head = head.next {
		if head.IsLast() {
			return head
		}
	}
	//panic("back not found")
	return
}
