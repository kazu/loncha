// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lonacha/list_head is like a kernel's LIST_HEAD
// list_head is used by lonacha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
	"strings"
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

func (head *ListHead) isDeleted() (deleted bool) {
	/*defer func() {
		if perr := recover(); perr != nil {
			fmt.Printf("\nisDelete(): recover %+v\n", head)
		}
	}()*/
	if head == nil {
		panic("isDelete invalid")
	}
	nptr := unsafe.Pointer(head.next)
	next := atomic.LoadPointer(&nptr)
	if next == nil {
		return false
	}

	if uintptr(next)&1 > 0 {
		//if ptr&1 != 0 {
		return true
	}
	return false
}

func (list *ListHead) DeleteMarked() {

	head := list.Front()
	elm := head
	old := elm
	before_len := head.Len()
	before_dump := head.DumpAll()
	before_head_pp := head.Pp()

	defer func() {
		new_dump := head.DumpAll()
		//if len(new_dump) > len(before_dump) {

		fmt.Printf("before %s \n%s\nafter(%s)\n%s\n",
			before_head_pp, before_dump,
			head.Pp(), new_dump)
		//}
	}()

	if head.Len() == 1 {
		fmt.Printf("debug start\n")
	}

	for {
		// mark
		old = elm
		elm = elm.next
		if old == elm {
			return
		}
		/*
			if !old.isDeleted() && !elm.isDeleted() {
				//elm.prev = old
				atomic.StorePointer(
					(*unsafe.Pointer)(unsafe.Pointer(&elm.prev)),
					unsafe.Pointer(old))
			}*/

		if elm.isDeleted() {
			success := elm.deleteDirect(old)
			//if elm.deleteDirect(old) {
			//				old = elm
			//			} else {
			fmt.Printf("elm deleteDirect prev=%s e=%s succ=%v\n", old.Pp(), elm.Pp(), success)
			elm = head
			//}
		}
		if before_len < head.Len() {
			fmt.Printf("increament ?\n")
		}

	}

}

func (head *ListHead) Next() (next *ListHead) {

	if !MODE_CONCURRENT {
		next = head.next
		return
	}

	cptr := unsafe.Pointer(head)
	curPtr := atomic.LoadPointer(&cptr)
	//_ = cur

	ptr := unsafe.Pointer(head.next)
	nextPtr := atomic.LoadPointer(&ptr)

	cur := (*ListHead)(curPtr)
	next = (*ListHead)(nextPtr)

	if cur == next {
		return
	}

	if cur.isDeleted() {
		nextPtr = unsafe.Pointer(uintptr(nextPtr) ^ 1)
		next = (*ListHead)(nextPtr)
		if cur == next {
			return
		}
		next = next.Next()
	}
	return

}

func listAdd(new, prev, next *ListHead) {
	if prev != next {
		next.prev, new.next, new.prev, prev.next = new, next, prev, new
	} else {
		prev.next, new.prev = new, prev
	}
}

func listAddWitCas(new, prev, next *ListHead) (err error) {
	if prev != next {
		new.next = next
	}

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(next),
		unsafe.Pointer(new)) {
		if prev != next {
			next.prev, new.prev = new, prev
		} else {
			new.prev = prev
		}
		return
	}
	return //errors.New("cas conflict")
	return fmt.Errorf("listAddWithCas() fail retry: new=%s prev=%s next=%s",
		new.Pp(),
		prev.Pp(),
		next.Pp())

}

func (head *ListHead) Add(new *ListHead) {
	if MODE_CONCURRENT {
		for true {
			err := listAddWitCas(new, head, head.next)
			if err == nil {
				break
			}
			fmt.Printf("err=%s\n", err.Error())
		}
		return
	}
	listAdd(new, head, head.next)
}

func (l *ListHead) MarkForDelete() (err error) {

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&l.next)),
		unsafe.Pointer(l.next),
		unsafe.Pointer(uintptr(unsafe.Pointer(l.next))|1)) {
		return
	}
	return errors.New("cas conflict(fail mark)")
}

func (l *ListHead) DeleteWithCas(prev *ListHead) (err error) {
	use_mark := true

	head := l.Front()

	defer func() {
		if err == nil {
			if ContainOf(head, l) {
				panic("????!!!")
			}
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
		err = l.MarkForDelete()
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

func (l *ListHead) deleteDirect(oprev *ListHead) (success bool) {
	prev := l.prev

	if oprev != nil {
		prev = oprev
	}

	success = false
	defer func() {
		if success {
			l.next, l.prev = l, l
		}
	}()

	if l.IsLast() {
		if atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
			unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1),
			unsafe.Pointer(prev)) {
			success = true
			return
		}
		return
	}

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(l),
		unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1)) {
		//		unsafe.Pointer(l.prev)) {
		success = true
		if l.IsLast() {
			panic("????")
		} else {
			//l.Next().prev = l.prev

			return
		}
	}

	return
}

func (l *ListHead) Pp() string {

	return fmt.Sprintf("%p{prev: %p, next:%p, len: %d}", l, l.prev, l.next, l.Len())
}

func (l *ListHead) Delete() (result *ListHead) {
	/*
		defer func() {
			if perr := recover(); perr != nil {
				fmt.Printf("panic: retry l=%p\n", l)
				if !MODE_CONCURRENT {
					panic(perr)
				}
				for true {
					err := l.DeleteWithCas()
					if err == nil {
						break
					}
					fmt.Printf("Delete() err=%s\n", err.Error())
				}
				l.Init()
				result = l.next
			}

		}()
	*/
	if MODE_CONCURRENT {
		for true {
			err := l.DeleteWithCas(l.prev)
			if err == nil {
				break
			}
			fmt.Printf("err=%s\n", err.Error())
			return nil
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
	return l.Next() == l
}

func (l *ListHead) IsFirst() bool {
	return l.prev == l
}

func (l *ListHead) Len() (cnt int) {

	cnt = 0
	for s := l; s.Prev() != s; s = s.Prev() {
		cnt++
	}

	for s := l; s.Next() != s; s = s.Next() {
		cnt++
	}

	return cnt
}

func (l *ListHead) Front() (head *ListHead) {

	for head = l; head.Prev() != head; head = head.Prev() {
		if head.IsFirst() {
			return head
		}
	}
	//panic("front not found")
	return
}

func (l *ListHead) Back() (head *ListHead) {

	for head = l; head.Next() != head; head = head.Next() {
		if head.IsLast() {
			return head
		}
	}
	//panic("back not found")
	return
}

type Cursor struct {
	Pos *ListHead
}

func (l *ListHead) Cursor() Cursor {

	return Cursor{Pos: l}
}

func (cur *Cursor) Next() bool {

	if cur.Pos == cur.Pos.Next() {
		return false
	}
	cur.Pos = cur.Pos.Next()
	return true

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

func (head *ListHead) DumpAll() string {

	c := head.Cursor()
	cnt := 1
	var b strings.Builder
	for c.Next() {
		for i := 0; i < cnt; i++ {
			b.WriteString("\t")
		}
		b.WriteString(c.Pos.Pp())
		b.WriteString("\n")
	}

	return b.String()
}
