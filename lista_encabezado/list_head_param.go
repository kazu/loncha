// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
	"sync/atomic"
	"unsafe"
)

type List interface {
	Offset() uintptr
	PtrListHead() *ListHead
}

// ElementOf .. get struct pointer from struct.ListHead
func ElementOf(l List, head *ListHead) unsafe.Pointer {
	if head == nil || l == nil {
		return nil
	}

	return unsafe.Pointer(uintptr(unsafe.Pointer(head)) - l.Offset())
}


// Add ... Add list 
//     support lista_encabezado
func (head *ListHead) AddElement(nList List) *ListHead {
	n := nList.PtrListHead()
	head.Add(n)
	return n
}

func (head *ListHead) Add(new *ListHead) *ListHead {
	head.add(new)
	return new
}

func (head *ListHead) add(new *ListHead) {
	if MODE_CONCURRENT {
		for true {
			//err := listAddWitCas(new, head, (*ListHead)(headNext))
			err := listAddWitCas(new,
				head,
				(*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next)))))
			if err == nil {
				break
			}
			fmt.Printf("Add(): retry err=%s\n", err.Error())
		}
		return
	}
	listAdd(new, head, head.next)
}

func (l *ListHead) DeleteElementWithCas(pList List) (err error) {
	return l.DeleteWithCas(pList.PtrListHead())
}

func (l *ListHead) DeleteWithCas(prev *ListHead) (err error) {	
	return l.deleteWithCas(prev)
}

func (l *ListHead) deleteWithCas(prev *ListHead) (err error) {	
	use_mark := true

	head := l.Front()
	_ = head
	defer func() {
		if err == nil {
			//if ContainOf(head, l) {
			//	panic("????!!!")
			//}
		}
	}()

	/*
		defer func() {

			if perr := recover(); perr != nil {
				fmt.Printf("panic: retry l=%p\n", l)
				err = fmt.Errorf("panic: retry l=%p\n", l)
				return
			}
			if err == nil {
				fmt.Printf("Success: l=%p\n", l)
				l.Init()
			}

			return
		}()
	*/
	if l.IsFirst() {
		//l.next.prev = l.next
		//panic("first element cannot delete")
		return errors.New("first element cannot delete")
	}
	if use_mark {
		err = l.MarkForDelete() // FIXME: race condition 79
		if err != nil {
			return err
		}
	}

	if l.deleteDirect(prev) {
		return
	} else {
		//l.DeleteMarked()
		return errors.New("retry from list first")
	}

	return fmt.Errorf("Delete() fail retry: l.prev=%s l=%s l.prev.isDeleted=%v l.IsLast()=%v",
		l.prev.Pp(),
		l.Pp(),
		l.prev.isDeleted(),
		l.IsLast())
}

//func ContainOf(head, elm *ListHead) bool {
func ElementIsContainOf(hList, l List) bool {
	return ContainOf(hList.PtrListHead(), l.PtrListHead())
}

func ContainOf(head, elm *ListHead) bool {	
	
	c := head.Cursor()

	for c.Next() {
		if c.Pos == elm {
			return true
		}
	}

	return false
}