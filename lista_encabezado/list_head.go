// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"unsafe"
)

var (
	MODE_CONCURRENT      bool = false
	PANIC_NEXT_IS_MARKED bool = false
)

var (
	// ErrDeleteFirst is undocumnted
	ErrDeleteFirst error = errors.New("first element cannot delete")
	// ErrListNil is undocumnted
	ErrListNil error = errors.New("list is nil")
	// ErrEmpty is undocumnted
	ErrEmpty error = errors.New("list is empty")
	// ErrMarked is undocumnted
	ErrMarked error = errors.New("element is marked")
)

type ListHead struct {
	prev *ListHead
	next *ListHead
}

type ListHaedError struct {
	head *ListHead
	err  error
}

func (le ListHaedError) Error() string {
	return le.err.Error()
}
func (le ListHaedError) List() *ListHead {
	return le.head
}

func ListWithError(head *ListHead, err error) ListHaedError {
	return ListHaedError{head: head, err: err}
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

type BoolAndError struct {
	t bool
	e error
}

func MakeBoolAndError(t bool, e error) BoolAndError {
	return BoolAndError{t: t, e: e}
}

func (head *ListHead) isMarkedForDeleteWithoutError() (deleted bool) {

	return MakeBoolAndError(head.isMarkedForDelete()).t
}

func (head *ListHead) isMarkedForDelete() (deleted bool, err error) {

	if head == nil {
		return false, ErrListNil
	}
	next := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next)))

	if next == nil {
		return false, errors.New("next is nil")
	}

	if uintptr(next)&1 > 0 {
		return true, nil
	}
	return false, nil

}

func (list *ListHead) DeleteMarked() (err error) {

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

			err = errors.New("fail cas for DeleteMarked loop")
			continue
		}

		if old == elm {
			//fmt.Printf("Info: DeleteMarked END %p\n", head)
			return err
		}
		isMaredForDeleted, err := elm.isMarkedForDelete()

		if isMaredForDeleted {
			elm.deleteDirect(old)
			elm = head
		} else if err != nil {
			err = fmt.Errorf("cannot DeleteMarked invalid becaouse invalid list err=%s", err.Error())
			return err
		}
	}

}

func (head *ListHead) Next() (nextElement *ListHead) {
	//MENTION: ignore error. should use NextWithError()
	return head.NextWithError().head
}

func (head *ListHead) NextWithError() ListHaedError {

	if !MODE_CONCURRENT {
		return ListHaedError{head.next, nil}
	}
	return ListWithError(head.Next1())
}

func (head *ListHead) Next1() (nextElement *ListHead, err error) {
	defer func() {
		if nextElement == nil {
			nextElement = head
		}
	}()

retry:
	nextElement, err = head.next3()
	if head.next != nextElement && nextElement != nil {
		goto retry
	}
	if nextElement != nil && nextElement.prev != head {
		//fmt.Printf("repaore invalid nextElement.prev cur=%p next=%p next.prev=%p\n", head, nextElement, nextElement.prev)

		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&nextElement.prev)),
			unsafe.Pointer(head))

		goto retry
	}

	return
}

// return nil on last of list
// Deprecated: not use next1()
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

		if nextElement.isMarkedForDeleteWithoutError() {
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

func (head *ListHead) next3() (next *ListHead, err error) {

	headNext := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next)))

	if unsafe.Pointer(head) == headNext {
		return nil, ErrEmpty
	}
	if unsafe.Pointer(head) == unsafe.Pointer(uintptr(headNext)^1) {
		return nil, ErrMarked
	}

	if head.isMarkedForDeleteWithoutError() {
		if PANIC_NEXT_IS_MARKED {
			head.isMarkedForDeleteWithoutError()
			panic("next3 must not isDeleted()")
		}

		//fmt.Fprintf(os.Stderr, "WARN: return marked value because next is marked\n")
		return nil, ErrMarked

	}
	if (*ListHead)(headNext).isMarkedForDeleteWithoutError() {
		err = head.DeleteMarked()
	}

	return (*ListHead)(headNext), err

}

// Deprecated: next2 should be used
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

		if nextElement.isMarkedForDeleteWithoutError() {
			head.DeleteMarked()
			goto RESTART
		} else {
			return nextElement
		}

	}

	return nil

}

// Deprecated: Next0 ... should be used
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
	if cur.isMarkedForDeleteWithoutError() {
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
	if next == nil {
		fmt.Printf("???")
	}

	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(next),
		unsafe.Pointer(new)) {
		if next == nil {
			fmt.Printf("???")
		}

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
	if prev.isMarkedForDeleteWithoutError() {
		return ErrMarked
	}

	//return errors.New("cas conflict")
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
				//panic(fmt.Sprintf("fail CAS prev.next.prev=%p  != l=%p prev.next=%p l.next=%p  l.prev=%p  ", prev.next.prev, l, prev.next, l.next, l.prev))
				fmt.Printf("WARN: fail CAS prev.next.prev=%p  != l=%p prev.next=%p l.next=%p  l.prev=%p  ", prev.next.prev, l, prev.next, l.next, l.prev)

			}
			// atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&prev.next.prev)),
			// 	unsafe.Pointer(l.prev))

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
	retry := 100
	if MODE_CONCURRENT && l.IsFirst() {
		l.deleteFirst()
		goto ENSURE
	}

	if MODE_CONCURRENT {

		for ; retry > 0; retry-- {
			//			err := l.deleteWithCas(l.prev)
			err := l.deleteWithCas((*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))))
			if err == nil {
				break
			}
			if err == ErrDeleteFirst {
				l.deleteFirst()
				goto ENSURE
			}
			fmt.Printf("retry=%d err=%s\n", retry, err.Error())
		}
		if retry <= 0 {
			panic("fail")
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
ENSURE:
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
	if !l.isMarkedForDeleteWithoutError() {
		return l.next == l
	}

	return unsafe.Pointer(l) == unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1)

}

func (l *ListHead) IsFirst() bool {
	prev := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))

	return prev == unsafe.Pointer(l) // l.prev == l // FIXME: race condition ? :350, 358
}

func (l *ListHead) Len() (cnt int) {

	retry := 10
RETRY:
	cnt = 0
	var isMarked bool
	var err error
	if retry < 1 {
		return -1
	}

	for s := l; s.Prev() != s; s = s.Prev() {
		cnt++
	}

	isMarked, err = l.isMarkedForDelete()

	if isMarked {
		retry--
		goto RETRY
		//return cnt
	}
	if err != nil {
		return -1
	}

	for s := l; s.Next() != s; s = s.Next() {
		cnt++
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
		if prev.isMarkedForDeleteWithoutError() {
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

func (head *ListHead) Validate() error {

	if !head.IsFirst() {
		return errors.New("list not first element")
	}

RETRY:

	hasCur := map[*ListHead]int{}
	hasNext := map[*ListHead]int{}
	cnt := 0

	for cur, next := head, head.Next(); !next.IsLast(); cur, next = cur.Next(), next.Next() {
		if _, ok := hasCur[cur]; ok {
			return errors.New("this list is partial loop")
		}
		if _, ok := hasNext[next]; ok {
			return errors.New("this list is partial loop")
		}
		hasCur[cur] = cnt
		hasNext[next] = cnt

		if cur.isMarkedForDeleteWithoutError() {
			err := cur.DeleteMarked()
			if err != nil {
				return err
			}
			goto RETRY
		}
		if next.isMarkedForDeleteWithoutError() {
			err := cur.DeleteMarked()
			if err != nil {
				return err
			}
			goto RETRY
		}
		if cur.next != next {
			fmt.Printf("invalid cur.next  hasCur[cur.next]=%v hasCur[next]=%v  hasNext[cur.next]=%v hasNext[next])=%v\n",
				hasCur[cur.next], hasCur[next], hasNext[cur.next], hasNext[next])
			return fmt.Errorf("invalid cur.next  hasCur[cur.next]=%v hasCur[next]=%v  hasNext[cur.next]=%v hasNext[next])=%v",
				hasCur[cur.next], hasCur[next], hasNext[cur.next], hasNext[next])
		}
		if next.prev != cur {
			// fmt.Printf("cnt=%d cur=%p next=%p next.prev=%p cur.next=%p\n",
			// 	cnt, cur, next, next.prev, cur.next)
			// fmt.Printf("invalid cnt=%d next.prev  hasCur[next.prev]=%v hasCur[cur]=%v hasNext[next.prev]=%v hasNext[cur]=%v\n",
			// 	cnt, hasCur[next.prev], hasCur[cur], hasNext[next.prev], hasNext[cur])
			// atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&next.prev)),
			// 	unsafe.Pointer(cur))
			// goto RETRY
			return fmt.Errorf("invalid next.prev  hasCur[next.prev]=%v hasCur[cur]=%v hasNext[next.prev]=%v hasNext[cur]=%v",
				hasCur[next.prev], hasCur[cur], hasNext[next.prev], hasNext[cur])
		}
		cnt++
	}
	return nil

}
