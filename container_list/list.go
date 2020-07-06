package list

// ListEntry is a base of http://golang.org/pkg/container/list/
// this is tuning performancem, reduce heap usage.
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"github.com/cheekybits/genny/generic"
	"github.com/kazu/loncha/list_head"
    "unsafe"
    "errors"
)

// ListEntry ... liked-list like a kernel list head
type ListEntry generic.Type

func New() (l *ListEntry) {
	l := &ListEntry{}
	l.Init()
	return
} 

func (d *ListEntry) Init() {
	d.ListHead.Init()
}

// Next ... returns the next list element or nil.
func (d *ListEntry) Next() *ListEntry {
	if d.ListHead.Next() == nil {
		panic(errors.New("d.next is nil"))
	}
	return (*ListEntry)(unsafe.Pointer(uintptr(unsafe.Pointer(d.ListHead.Next())) - unsafe.Offsetof(d.ListHead)))
}
// Prev ... returns the previous list element or nil.
func (d *ListEntry) Prev() *ListEntry {
	if d.ListHead.Next() == nil {
		panic(errors.New("d.prev is nil"))
	}
	return (*ListEntry)(unsafe.Pointer(uintptr(unsafe.Pointer(d.ListHead.Prev())) - unsafe.Offsetof(d.ListHead)))
}

// NewListEntryList ... New returns an initialized list.
func NewListEntryList(h *ListEntry) *ListEntry {
	h.Init()
	return h
}
// Len ... size of list
func (d *ListEntry) Len() int {
	return d.ListHead.Len()
}

// Add ... insert n after d
func (d *ListEntry) Add(n *ListEntry)  *ListEntry {
	d.ListHead.Add(&n.ListHead)
	return n
}
// Delete ... delete d from linked-list
func (d *ListEntry) Delete() *ListEntry {
	ptr := d.ListHead.Delete()
	return (*ListEntry)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) - unsafe.Offsetof(d.ListHead)))
}

// Remove ... Alias of Delete()
func (d *ListEntry) Remove() *ListEntry {
	return d.Delete()
}

// ContainOf ... find same entry of ptr
func (d *ListEntry) ContainOf(ptr *list_head.ListHead) *ListEntry {
	return (*ListEntry)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) - unsafe.Offsetof(d.ListHead)))
}

// Front ... first value of ListEntry
func (d *ListEntry) Front() *ListEntry {
	return d.ContainOf(d.ListHead.Front())
}

// Back ... last entry of linked-list
func (d *ListEntry) Back() *ListEntry {
	return d.ContainOf(d.ListHead.Back())
}

// PushFront ... inserts a new value v at the front of list l and returns e.
func (d *ListEntry) PushFront(v *ListEntry) *ListEntry {
	front := d.Front()
	v.Add(front)
	return v
}


// PushBack ... inserts a new element e with value v at the back of list l and returns e.
func (l *ListEntry) PushBack(v *ListEntry) *ListEntry {
	last := l.Back()
	last.Add(v)
	return v
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element of l, the list is not modified.
func (l *ListEntry) InsertBefore(v *ListEntry) *ListEntry {
	l.Prev().Add(v)
	return v
}

// InsertAfter ... inserts a new element e with value v immediately after mark and returns e.
// If mark is not an element of l, the list is not modified.
func (l *ListEntry) InsertAfter(v *ListEntry) *ListEntry {
	l.Next().Add(v)
	return v
}


// MoveToFront ... moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
func (l *ListEntry) MoveToFront(v *ListEntry) *ListEntry {
	v.Remove()
	return l.PushFront(v)
}

// MoveToBack ... moves element e to later of list l.
// If e is not an element of l, the list is not modified.
func (l *ListEntry) MoveToBack(v *ListEntry) *ListEntry {
	v.Remove()
	return l.PushBack(v)
}


// MoveBefore ... moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
func (l *ListEntry) MoveBefore(v *ListEntry) *ListEntry {
	v.Remove()
	l.Prev().Add(v)
	return v
}

// MoveAfter ... moves element e to its new position after mark.
// If e is not an element of l, or e == mark, the list is not modified.
func (l *ListEntry) MoveAfter(v *ListEntry) *ListEntry {
	v.Remove()
	l.Add(v)
	return v
}

func (l *ListEntry) PushBackList(other *ListEntry) {
	l.Back().Add(other)
	return 
}

func (l *ListEntry) PushFrontList(other *ListEntry) {
	other.PushBackList(l)
	return
}


func (l *ListEntry) Each(fn func(e *ListEntry)) {

	cur := l.Cursor()

	for cur.Next() {
		fn(l.ContainOf(cur.Pos))
	}

}
