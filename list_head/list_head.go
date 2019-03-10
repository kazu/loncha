// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lonacha/list_head is like a kernel's LIST_HEAD
// list_head is used by lonacha/gen/containers_list
package list_head

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

func (head *ListHead) Next() *ListHead {
	return head.next
}

func listAdd(new, prev, next *ListHead) {
	if prev != next {
		next.prev, new.next, new.prev, prev.next = new, next, prev, new
	} else {
		prev.next, new.prev = new, prev
	}

}

func (head *ListHead) Add(new *ListHead) {
	listAdd(new, head, head.next)
}

func (l *ListHead) Delete() *ListHead {

	if l.Len() > 2 {
		l.next.prev, l.prev.next = l.prev, l.next
	} else {
		l.prev.next = l.prev
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

	cnt = 0
	for head := l.Front(); head.next != head; head = head.next {
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
