// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
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
	MODE_CONCURRENT      bool = false
	PANIC_NEXT_IS_MARKED bool = false
)

type ListHead struct {
	prev *ListHead
	next *ListHead
}

func GetConcurrentMode() bool {
	return MODE_CONCURRENT
}

func (head *ListHead) Init() {
	head.prev = head
	head.next = head
}

func (head *ListHead) Prev() *ListHead {
	prev := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev)))
	return (*ListHead)(prev)
}

func (head *ListHead) DirectNext() *ListHead {
	return head.next
}

func (head *ListHead) PtrNext() **ListHead {
	//return atomic.LoadPointer(&head.next)
	return &head.next
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
	next := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next)))

	if next == nil {
		panic("isDelete next is nil")
		return false
	}

	if uintptr(next)&1 > 0 {
		return true
	}
	return false

}

func (list *ListHead) DeleteMarked() {

	head := list.Front()
	//fmt.Printf("Info: DeleteMarked START %p\n", head)
	elm := head
	old := elm

	for {
		// mark
		old = elm
		//atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&elm)), unsafe.Pointer(elm.next)) // elm = elm.next // FIXME: race condition 413, 85
		if !atomic.CompareAndSwapPointer(
			(*unsafe.Pointer)(unsafe.Pointer(&elm)),
			unsafe.Pointer(old),
			unsafe.Pointer(elm.next)) {

			fmt.Printf("WARN: fail cas for DeleteMarked loop\n")
			continue
		}

		if old == elm {
			//fmt.Printf("Info: DeleteMarked END %p\n", head)
			return
		}

		if elm.isDeleted() {
			elm.deleteDirect(old)
			//fmt.Printf("Info: DeleteMarked STOP/RESTART %p\n", head)
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
retry:
	nextElement = head.next3()
	if head.next != nextElement && nextElement != nil {
		//fmt.Printf("head.next=%s nextElement=%s\n", head.Pp(), nextElement.Pp())
		goto retry
	}
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

	headNext := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next)))

	if unsafe.Pointer(head) == headNext {
		return nil
	}
	if unsafe.Pointer(head) == unsafe.Pointer(uintptr(headNext)^1) {
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
	if (*ListHead)(headNext).isDeleted() {
		head.DeleteMarked()
	}

	return (*ListHead)(headNext)

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

//  prev ---------------> next
//        \--> new --/
//   prev --> next     prev ---> new
func listAddWitCas(new, prev, next *ListHead) (err error) {
	if prev != next {
		//new.next = next
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&new.next)),
			unsafe.Pointer(next))
	}

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(next),
		unsafe.Pointer(new)) {
		if prev != next {
			//next.prev, new.prev = new, prev
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&next.prev)),
				unsafe.Pointer(new))
			//new.prev = prev
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&new.prev)),
				unsafe.Pointer(prev))
		} else {
			//new.prev = prev
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&new.prev)),
				unsafe.Pointer(prev))
		}
		return
	}
	return //errors.New("cas conflict")
	return fmt.Errorf("listAddWithCas() fail retry: new=%s prev=%s next=%s",
		new.Pp(),
		prev.Pp(),
		next.Pp())

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

func (l *ListHead) deleteDirect(oprev *ListHead) (success bool) {
	prev := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))) // prev := l.prev // FIXME: race condition 452
	if oprev != nil {
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&prev)), unsafe.Pointer(oprev)) // prev = oprev
	}

	success = false
	defer func() {
		if success {
			//l.next, l.prev = l, l
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)),
				unsafe.Pointer(l))
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)),
				unsafe.Pointer(l))
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
	// l->next is marked
	//  prev -> l -> l.next
	//  prev -----> l.next
	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(l),
		unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1)) {
		//		unsafe.Pointer(l.prev)) {
		success = true
		if l.isLastWithMarked() {
			panic("no delete l->next  mark")
		} else {
			// prev.next.prev = l.prev
			if !atomic.CompareAndSwapPointer(
				(*unsafe.Pointer)(unsafe.Pointer(&prev.next.prev)),
				unsafe.Pointer(l),
				unsafe.Pointer(l.prev)) {
				panic("fail remove")
			}

			return
		}
	}

	return
}

func (l *ListHead) Pp() string {

	return fmt.Sprintf("%p{prev: %p, next:%p, len: %d}",
		l,
		atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev))), //l.prev,
		atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.next))), //l.next,
		l.Len()) // FIXME: race condition 350
}

func (l *ListHead) P() string {

	return fmt.Sprintf("%p{prev: %p, next:%p}",
		l,
		atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev))), //l.prev,
		atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)))) //l.next)
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
					err := l.deleteWithCas()
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
			//			err := l.deleteWithCas(l.prev)
			err := l.deleteWithCas((*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))))
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
	// l.next, l.prev = l, l // FIXME: race condition 56
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)), unsafe.Pointer(l))
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)), unsafe.Pointer(l))
	return (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)))) //l.next

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
	prev := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))

	return prev == unsafe.Pointer(l) // l.prev == l // FIXME: race condition ? :350, 358
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
	isInfinit := map[*ListHead]bool{}

	for head = l; head.Prev() != head; head = head.Prev() {

		if head.IsFirst() {
			return head
		}
		if isInfinit[head] {
			panic("infinit loop")
		}
		isInfinit[head] = true
	}
	//panic("front not found")
	return
}

func (l *ListHead) Back() (head *ListHead) {
	isInfinit := map[*ListHead]bool{}

	for head = l; head.Next() != head; head = head.Next() {
		if head.IsLast() {
			return head
		}
		if isInfinit[head] {
			panic("infinit loop")
		}
		isInfinit[head] = true
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
