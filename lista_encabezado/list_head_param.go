// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

type List interface {
	Offset() uintptr
	PtrListHead() *ListHead
	FromListHead(*ListHead) List
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
		retry := 0
		var err error
		for true {
			//err := listAddWitCas(new, head, (*ListHead)(headNext))
			err = listAddWitCas(new,
				head,
				(*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next)))))
			if err == nil {
				// if retry > 0 {
				// 	fmt.Printf("add(): success retry=%d\n", retry)
				// }
				break
			}
			if err == ErrMarked {
				err = head.DeleteMarked()
			}
			//fmt.Printf("Add(): retry=%d err=%s\n", retry, err.Error())
			retry++
		}
		if err != nil {
			fmt.Printf("add(): over retry retry=%d err=%s\n", retry, err.Error())
		}
		return
	}
	listAdd(new, head, head.next)
}

func (head *ListHead) Join(new *ListHead) {
	if new.Empty() {
		head.Add(new)
		return
	}
	list := head

	for cur, next := new, new.Next(); !cur.IsLast(); cur, next = next, next.Next() {
		cur.Delete()
		list = list.Add(cur)
		if next.IsLast() {
			next.Delete()
			list.Add(next)
			break
		}
	}

}

func (l *ListHead) DeleteElementWithCas(pList List) (err error) {
	return l.DeleteWithCas(pList.PtrListHead())
}

func (l *ListHead) DeleteWithCas(prev *ListHead) (err error) {
	return l.deleteWithCas(prev)
}

func (l *ListHead) deleteFirst() (err error) {

	if l.Next() == l {
		return nil
	}

	if l.Next().IsLast() {
		l.Next().Delete()
		return nil
	}

	next := l.Next()
	nextNext := next.Next()

	next.Delete()
	next.Init()
	next.Add(nextNext)
	l.Init()
	return nil

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
		return ErrDeleteFirst
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
		err = l.DeleteMarked()
		if err != nil {
			return fmt.Errorf("retry from list first. fail DeleteMarked() err=%s", err.Error())
		}
		return nil
	}

	return fmt.Errorf("Delete() fail retry: l.prev=%s l=%s l.prev.isDeleted=%v l.IsLast()=%v",
		l.prev.Pp(),
		l.Pp(),
		l.prev.isMarkedForDeleteWithoutError(),
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
