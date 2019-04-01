// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lonacha/list_head is like a kernel's LIST_HEAD
// list_head is used by lonacha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"unsafe"
)

var (
	MODE_CONCURRENT      = false
	PANIC_NEXT_IS_MARKED = false
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

func IsMarked(elm *ListHead) (marked bool) {

	nptr := unsafe.Pointer(elm)
	next := atomic.LoadPointer(&nptr)

	if next == nil {
		panic("isDelete next is nil")
		return false
	}

	if uintptr(next)&1 > 0 {
		return true
	}
	return false

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
	return IsMarked(head.next)

}

func (list *ListHead) DeleteMarked() {

	head := list.Front()
	elm := head
	old := elm

	for {
		// mark
		old = elm
		elm = elm.next
		if old == elm {
			return
		}

		if elm.isDeleted() {
			elm.deleteDirect(old)
			elm = head
			//}
		}

	}

}

func (head *ListHead) Next() (nextElement *ListHead) {

	if !MODE_CONCURRENT {
		return head.next
	}

	return head.Next1()
}

func (head *ListHead) Next1() (nextElement *ListHead) {
	defer func() {
		if nextElement == nil {
			nextElement = head
		}

		//fmt.Printf("\thead(%s).next1() => %s\n", head.P(), nextElement.P())

	}()

	//nextElement = head.next1()
	nextElement = head.next3()
	return
}

// return nil on last of list
func (head *ListHead) next1() (nextElement *ListHead) {

	uptr := unsafe.Pointer(head.next)
	next := atomic.LoadPointer(&uptr)

	hptr := unsafe.Pointer(head)
	pHead := atomic.LoadPointer(&hptr)

	EqualWithMark := func(src, dst unsafe.Pointer) bool {
		if src == nil {
			return true
		}

		if uintptr(src) == uintptr(dst) {
			return true
		}

		if uintptr(src) > uintptr(dst) && uintptr(src)-uintptr(dst) <= 1 {
			return true
		}
		if uintptr(src) < uintptr(dst) && uintptr(dst)-uintptr(src) <= 1 {
			return true
		}
		return false
	}

	for !EqualWithMark(next, pHead) {
		//	for next != pHead {

		if uintptr(next)&1 > 0 {
			nextElement = (*ListHead)(unsafe.Pointer(uintptr(next) ^ 1))
			//Log(true).Debug("list.next1() is marked skip ", zap.String("head", head.P()))
			return nextElement.next1()
		}
		nextElement = (*ListHead)(next)

		if nextElement.isDeleted() {
			pHead = atomic.LoadPointer(&uptr)
			atomic.CompareAndSwapPointer(&uptr, next, unsafe.Pointer(nextElement.next1()))
			next = atomic.LoadPointer(&uptr)
			if next != nil {
				// Log(true).Debug("list.next1() is marked(next loop) ",
				// 	zap.String("head", head.P()),
				// 	zap.String("next", ((*ListHead)(next)).P()),
				// )
			} else {
				// Log(true).Debug("list.next1() is marked(next loop) ",
				// 	zap.String("head", head.P()),
				// 	zap.String("next", "nil"),
				// )
			}
		} else {
			// Log(true).Debug("list.next1() not marking ",
			// 	zap.String("head", head.P()),
			// 	zap.String("next", nextElement.P()))

			return nextElement
		}

	}

	// Log(true).Debug("list.next1() last position ",
	// 	zap.String("head", head.P()),
	// )

	return nil
}

func (head *ListHead) next3() *ListHead {

	if unsafe.Pointer(head) == unsafe.Pointer(head.next) {
		return nil
	}
	if unsafe.Pointer(head) == unsafe.Pointer(uintptr(unsafe.Pointer(head.next))^1) {
		return nil
	}

	if head.isDeleted() {
		if PANIC_NEXT_IS_MARKED {
			head.isDeleted()
			panic("next3 must not isDeleted()")
		}

		fmt.Fprintf(os.Stderr, "WARN: return marked value because next is marked\n")
		return nil

	}
	if head.next.isDeleted() {
		head.DeleteMarked()
	}

	return head.next

}

func (head *ListHead) next2() (nextElement *ListHead) {

RESTART:

	uptr := unsafe.Pointer(head.next)
	next := atomic.LoadPointer(&uptr)

	hptr := unsafe.Pointer(head)
	pHead := atomic.LoadPointer(&hptr)

	EqualWithMark := func(src, dst unsafe.Pointer) bool {
		if src == nil {
			return true
		}

		if uintptr(src) == uintptr(dst) {
			return true
		}

		if uintptr(src) > uintptr(dst) && uintptr(src)-uintptr(dst) <= 1 {
			return true
		}
		if uintptr(src) < uintptr(dst) && uintptr(dst)-uintptr(src) <= 1 {
			return true
		}
		return false
	}

	for !EqualWithMark(next, pHead) {
		//	for next != pHead {

		if uintptr(next)&1 > 0 {
			head.DeleteMarked()
			goto RESTART
			//nextElement = (*ListHead)(unsafe.Pointer(uintptr(next) ^ 1))
			//return nextElement.next1()
		}
		nextElement = (*ListHead)(next)

		if nextElement.isDeleted() {
			head.DeleteMarked()
			goto RESTART
		} else {
			return nextElement
		}

	}

	return nil

}

func (head *ListHead) Next0() (next *ListHead) {

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
	/*
		if next.isDeleted() {
			return next.Next()
		}
	*/
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
		/*
			newNext := unsafe.Pointer(new.next)
			atomic.StorePointer(&newNext, unsafe.Pointer(next))
			new.next = (*ListHead)(atomic.LoadPointer(&newNext))
			if new.next != next {
				panic(fmt.Sprintf("??? %v %v ", newNext, unsafe.Pointer(next)))
			}*/
	}

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(next),
		unsafe.Pointer(new)) {
		if prev != next {
			//next.prev, new.prev = new, prev
			nextPrev := unsafe.Pointer(next.prev)
			atomic.StorePointer(&nextPrev, unsafe.Pointer(new))
			next.prev = (*ListHead)(atomic.LoadPointer(&nextPrev))
			new.prev = prev
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
			headNext := unsafe.Pointer(head.next)
			err := listAddWitCas(new, head, (*ListHead)(atomic.LoadPointer(&headNext)))
			if err == nil {
				break
			}
			fmt.Printf("Add(): retry err=%s\n", err.Error())
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

	if l.isLastWithMarked() {
		if atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
			//unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1),
			unsafe.Pointer(l),
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
		if l.isLastWithMarked() {
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

func (l *ListHead) P() string {

	return fmt.Sprintf("%p{prev: %p, next:%p}", l, l.prev, l.next)
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

func (l *ListHead) isLastWithMarked() bool {
	//return l.Next() == l
	if !l.isDeleted() {
		return l.next == l
	}

	return unsafe.Pointer(l) == unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1)

}

func (l *ListHead) IsFirst() bool {
	return l.prev == l
}

func (l *ListHead) Len() (cnt int) {

	cnt = 0
	for s := l; s.Prev() != s; s = s.Prev() {
		cnt++
	}

	if l.isDeleted() {
		return cnt
	}

	for s := l; s.Next() != s; s = s.Next() {
		cnt++
		//fmt.Printf("\t\ts=%s cnt=%d\n", s.P(), cnt)
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
		b.WriteString(c.Pos.P())
		b.WriteString("\n")
	}

	return b.String()
}

func (head *ListHead) DumpAllWithMark() string {

	cnt := 1
	var b strings.Builder

	cur := head
	prev := cur

	EqualWithMark := func(src, dst unsafe.Pointer) bool {
		if src == nil {
			return true
		}

		if uintptr(src) == uintptr(dst) {
			return true
		}

		if uintptr(src) > uintptr(dst) && uintptr(src)-uintptr(dst) <= 1 {
			return true
		}
		if uintptr(src) < uintptr(dst) && uintptr(dst)-uintptr(src) <= 1 {
			return true
		}
		return false
	}
	for i := 0; i < cnt; i++ {
		b.WriteString("\t")
	}
	b.WriteString(cur.P())
	b.WriteString("\n")
	cnt++

	for {
		prev = cur
		//cur = prev.next
		if prev.isDeleted() {
			cur = (*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(prev.next)) ^ 1))
		} else {
			cur = prev.next
		}

		for i := 0; i < cnt; i++ {
			b.WriteString("\t")
		}
		b.WriteString(cur.P())
		b.WriteString("\n")

		if EqualWithMark(unsafe.Pointer(prev), unsafe.Pointer(cur)) {
			break
		}
		cnt++
	}

	return b.String()
}
