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
	next.prev, new.next, new.prev, prev.next = new, next, prev, new
}

func (head *ListHead) Add(new *ListHead) {
	listAdd(new, head, head.next)
}

func (l *ListHead) Delete() *ListHead {
	next := l.next

	l.prev.next, l.next.prev = l.next, l.prev
	l.next = nil
	l.prev = nil
	return next
}

func (l *ListHead) Empty() bool {
	return l.next == l
}

func (l *ListHead) IsLast() bool {
	return l.next == l
}
