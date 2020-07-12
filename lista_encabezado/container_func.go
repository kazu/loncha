// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package list_head ... like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

// Remove ... Alias of Delete()
func (l *ListHead) Remove() *ListHead {
	return l.Delete()
}

func (l *ListHead) InsertAfter(vl List) *ListHead {
	v := vl.PtrListHead()
	l.Next().add(v)
	return v
}

func (l *ListHead) InsertBefore(vl List) *ListHead {
	v := vl.PtrListHead()
	l.Prev().add(v)
	return v
}

// MoveAfter ... moves element e to its new position after mark.
// If e is not an element of l, or e == mark, the list is not modified.
func (l *ListHead) MoveAfter(vl List) *ListHead {
	v := vl.PtrListHead()
	v.Remove()
	l.add(v)
	return v
}

// MoveBefore ... moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
func (l *ListHead) MoveBefore(vl List) *ListHead {
	v := vl.PtrListHead()
	v.Remove()
	l.Prev().add(v)
	return v
}

// MoveToBack ... moves element e to later of list l.
// If e is not an element of l, the list is not modified.
func (l *ListHead) MoveToBack(vl List) *ListHead {
	v := vl.PtrListHead()
	v.Remove()
	return l.PushBack(vl)
}

// MoveToFront ... moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
func (l *ListHead) MoveToFront(vl List) *ListHead {
	v := vl.PtrListHead()
	v.Remove()
	return l.PushFront(vl)
}

// PushBack ... inserts a new element e with value v at the back of list l and returns e.
func (l *ListHead) PushBack(vl List) *ListHead {
	v := vl.PtrListHead()
	last := l.Back()
	last.add(v)
	return v
}

func (l *ListHead) PushBackList(oE List) {
	other := oE.PtrListHead()
	l.Back().add(other)
	return
}

// PushFront ... inserts a new value v at the front of list l and returns e.
func (l *ListHead) PushFront(vl List) *ListHead {
	v := vl.PtrListHead()
	front := l.Front()
	v.add(front)
	return v
}

func (l *ListHead) PushFrontList(oE List) {
	other := oE.PtrListHead()
	other.Back().add(l)
	return
}

func (l *ListHead) Each(fn func(e *ListHead)) {

	cur := l.Cursor()

	for cur.Next() {
		fn(cur.Pos)
	}

}
