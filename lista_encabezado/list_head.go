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

const (
	ErrTDeleteFirst = 1 << iota
	ErrTListNil
	ErrTEmpty
	ErrTMarked
	ErrTNextMarked
	ErrTNotAppend
	ErrTNotMarked
	ErrTCasConflictOnMark
	ErrTFirstMarked
	ErrTCasConflictOnAdd
	ErrTOverRetyry
	ErrTNoSafetyOnAdd
)

var (
	ErrDeleteFirst       error = NewError(ErrTDeleteFirst, errors.New("first element cannot delete"))
	ErrListNil           error = NewError(ErrTListNil, errors.New("list is nil"))
	ErrEmpty             error = NewError(ErrTEmpty, errors.New("list is empty"))
	ErrMarked            error = NewError(ErrTMarked, errors.New("element is marked"))
	ErrNextMarked        error = NewError(ErrTNextMarked, errors.New("next element is marked"))
	ErrNotAppend         error = NewError(ErrTNotAppend, errors.New("element cannot be append"))
	ErrNotMarked         error = NewError(ErrTNotMarked, errors.New("elenment cannot be marked"))
	ErrCasConflictOnMark error = NewError(ErrTCasConflictOnMark, errors.New("cas conflict(fail mark)"))
	ErrFirstMarked       error = NewError(ErrTFirstMarked, errors.New("first element is marked"))
	ErrNoSafetyOnAdd     error = NewError(ErrTFirstMarked, errors.New("element is not safety to append"))
)

type ListHeadError struct {
	Type uint16
	Info string
	error
}

type OptNewError func(e *ListHeadError)

func NewError(t uint16, err error, opts ...OptNewError) *ListHeadError {

	e := &ListHeadError{Type: t, error: err}

	for _, opt := range opts {
		opt(e)
	}
	return e
}

func Error(oe error, opts ...OptNewError) error {
	e, success := oe.(*ListHeadError)
	if !success {
		return oe
	}

	for _, opt := range opts {
		opt(e)
	}
	return e
}

func ErrorInfo(s string) OptNewError {

	return func(e *ListHeadError) {
		e.Info = s
	}
}

type ListHead struct {
	prev *ListHead
	next *ListHead
}

type ListHeadWithError struct {
	head *ListHead
	err  error
}

func (le ListHeadWithError) Error() string {
	return le.err.Error()
}
func (le ListHeadWithError) List() *ListHead {
	return le.head
}

func ListWithError(head *ListHead, err error) ListHeadWithError {
	return ListHeadWithError{head: head, err: err}
}

func GetConcurrentMode() bool {
	return MODE_CONCURRENT
}

func NewEmpty() *ListHead {
	empty := &ListHead{}
	empty.prev = empty
	empty.next = empty
	return empty
}

func (head *ListHead) Init() {
	if !MODE_CONCURRENT {
		head.prev = head
		head.next = head
		return
	}

	start := NewEmpty()
	end := NewEmpty()
	head.prev = start
	head.next = end

	start.next = head
	end.prev = head
}

func (head *ListHead) InitAsEmpty() {

	end := NewEmpty()

	head.prev = head
	head.next = end

	end.next = end
	end.prev = head

}

func (head *ListHead) Prev() *ListHead {
	prev := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))

	return prev
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

func (head *ListHead) isMarkedForDeleteWithoutError() (marked bool) {

	return MakeBoolAndError(head.isMarkedForDelete()).t
}

func (head *ListHead) isMarkedForDelete() (marked bool, err error) {

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

func (head *ListHead) Next() (nextElement *ListHead) {
	//MENTION: ignore error. should use NextWithError()
	return head.NextWithError().head
}

func (head *ListHead) NextWithError() ListHeadWithError {

	if !MODE_CONCURRENT {
		return ListHeadWithError{head.next, nil}
	}
	if head.isMarkedForDeleteWithoutError() && head.IsFirst() {
		prev := head.prev
		return prev.NextWithError()
	}

	return ListWithError(head.Next1())
}

func (head *ListHead) Next1() (nextElement *ListHead, err error) {
	defer func() {
		if nextElement == nil {
			nextElement = head
		}
	}()

	err = retry(100, func(retry int) (bool, error) {
		nextElement, err = head.next3()
		switch err {
		case ErrNextMarked:
			return false, Error(err, ErrorInfo("next marked"))
		case ErrMarked:
			if head.Prev() == head {
				return true, Error(err, ErrorInfo("head.Prev() == head"))
			}
			//nextElement, err = head.Prev().Next1()
			return false, Error(err, ErrorInfo("call head.Prev().Next1()"))
		}
		if head.next != nextElement && nextElement != nil {
			return false, nil
		}
		if nextElement != nil && nextElement.prev != head {
			AddRecoverState("recoverPrev")
			nextElement.recoverPrev(head)
			return false, nil
		}
		return true, Error(err, ErrorInfo("nextElement, err = head.next3()"))
	})
	if errList, ok := err.(*ListHeadError); ok && errList.Type == ErrTOverRetyry {
		_ = errList

	}
	return
}

func (head *ListHead) recoverPrev(prev *ListHead) {

	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev)),
		unsafe.Pointer(prev))

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
	if (*ListHead)(headNext).isMarkedForDeleteWithoutError() {
		//err = head.DeleteMarked()
		return nil, ErrNextMarked
	}

	if head.isMarkedForDeleteWithoutError() {
		if PANIC_NEXT_IS_MARKED {
			head.isMarkedForDeleteWithoutError()
			panic("next3 must not isDeleted()")
		}

		//fmt.Fprintf(os.Stderr, "WARN: return marked value because next is marked\n")
		return nil, ErrMarked

	}

	return (*ListHead)(headNext), err

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
	// backup for roolback
	oNewPrev := uintptr(unsafe.Pointer(new.prev))
	oNewNext := uintptr(unsafe.Pointer(new.next))
	rollback := func(new *ListHead) {
		StoreListHead(&new.prev, (*ListHead)(unsafe.Pointer(oNewPrev)))
		StoreListHead(&new.next, (*ListHead)(unsafe.Pointer(oNewNext)))
	}
	_ = rollback

	// new.prev -> prev, new.next -> next
	StoreListHead(&new.prev, prev)
	StoreListHead(&new.next, next)

	if Cas(&prev.next, next, new) &&
		Cas(&next.prev, prev, new) {
		return nil
	}

	rollback(new)
	return NewError(ErrTCasConflictOnAdd,
		fmt.Errorf("listAddWithCas() please retry: new=%s prev=%s next=%s", new.P(), prev.P(), next.P()))

}

func (l *ListHead) MarkForDelete() (err error) {

	if !l.canPurge() {
		return ErrNotMarked
	}

	mask := uintptr(^uint(0)) ^ 1

	var (
		ErrDeketeStep0 error = errors.New("fail step 0")
		ErrDeketeStep1 error = errors.New("fail step 1")
		ErrDeketeStep2 error = errors.New("fail step 2")
		ErrDeketeStep3 error = errors.New("fail step 3")
	)
	_, _ = ErrDeketeStep2, ErrDeketeStep3

	err = retry(100, func(retry int) (fin bool, err error) {
		prev1 := (*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(l.prev)) & mask))
		next1 := (*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(l.next)) & mask))

		prev := prev1
		next := next1

		if retry > 50 {
			fmt.Printf("retry > 50\n")

		}

		if !MarkListHead(&l.next, next) {
			AddRecoverState("remove: retry marked next")
			return false, ErrDeketeStep0
		}
		if !MarkListHead(&l.prev, prev) {
			AddRecoverState("remove: retry marked prev")
			return false, ErrDeketeStep1
		}

		prev2 := PrevNoM(l.prev)
		next2 := NextNoM(l.next)
		_, _ = prev2, next2

		prevs := []**ListHead{&prev1.next, &prev2.next}
		nexts := []**ListHead{&next1.prev, &next2.prev}

		t := false
		_ = t
		for i, pn := range prevs {
			_ = i
			if *pn != l {
				continue
			}
			next := next1
			if next.IsMarked() {
				next = next2
			}
			t = Cas(prevs[i], l, next)
		}

		for i, np := range nexts {
			_ = i
			if *np != l {
				continue
			}
			prev := prev1
			if prev.IsMarked() {
				prev = prev2
			}
			t = Cas(np, l, prev)
		}

		for i, toL := range append(prevs, nexts...) {
			_ = i
			if l == *toL {
				AddRecoverState("remove: found node to me")
				return false, ErrDeketeStep2
			}
		}

		return true, nil
	})

	if err != nil {
		_ = err
	}

	return err
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
	onext := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)))
	if onext == nil || uintptr(onext)&1 == 0 {
		fmt.Printf("deleteDirect(): l.next is not marked")
	}
	// l->next is marked
	//  prev -> l -> l.next
	//  prev -----> l.next
	mask := uintptr(^uint(0)) ^ 1
	if atomic.CompareAndSwapPointer(
		(*unsafe.Pointer)(unsafe.Pointer(&prev.next)),
		unsafe.Pointer(l),
		unsafe.Pointer(uintptr(unsafe.Pointer(l.next))&mask)) {
		//unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1)) {
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

// Delete ... delete
// Deprecated: Delete()
func (l *ListHead) Delete() (result *ListHead) {

	//retry := 100
	// if MODE_CONCURRENT && l.IsFirst() {
	// 	l.deleteFirst()
	// 	goto ENSURE
	// }

	if MODE_CONCURRENT {

		// for ; retry > 0; retry-- {
		// 	//			err := l.deleteWithCas(l.prev)
		// 	err := l.deleteWithCas((*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))))
		// 	if err == nil {
		// 		break
		// 	}
		// 	if err == ErrEmpty {
		// 		return l
		// 	}
		// 	if err == ErrDeleteFirst {
		// 		l.deleteFirst()
		// 		goto ENSURE
		// 	}
		// 	fmt.Printf("retry=%d err=%s\n", retry, err.Error())
		// }
		// if retry <= 0 {
		// 	panic("fail")
		// }
		err := l.MarkForDelete()
		if err != nil {
			panic(err.Error())
		}
		// delete marked element
		//_ = l.Prev().Next()
		return l
	}

	if l.IsFirst() {
		l.next.prev = l.next
	} else if l.IsLast() {
		l.prev.next = l.prev
	} else {
		l.next.prev, l.prev.next = l.prev, l.next
	}

	// l.next, l.prev = l, l // FIXME: race condition 56
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)), unsafe.Pointer(l))
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)), unsafe.Pointer(l))
	return (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.next)))) //l.next

}

func (l *ListHead) Empty() bool {
	if MODE_CONCURRENT {
		return l.prev == l || l.next == l
	}
	return l.next == l
}

func (l *ListHead) IsLast() bool {
	return l.Next().Empty()
}

func (l *ListHead) isLastWithMarked() bool {
	//return l.Next() == l
	if !l.isMarkedForDeleteWithoutError() {
		return l.next == l
	}

	return unsafe.Pointer(l) == unsafe.Pointer(uintptr(unsafe.Pointer(l.next))^1)

}

func (l *ListHead) IsFirst() bool {
	prev := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev))))
	return prev.Empty()

	//	return prev == unsafe.Pointer(l) // l.prev == l // FIXME: race condition ? :350, 358
}

func (l *ListHead) IsFirstMarked() bool {
	prev := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&l.prev)))

	return prev == unsafe.Pointer(l) // l.prev == l // FIXME: race condition ? :350, 358
}

func (l *ListHead) Len() (cnt int) {
	if MODE_CONCURRENT {
		return l.lenCc()
	}
	return l.len()
}

func (l *ListHead) lenCc() (cnt int) {
	cnt = 0
	var loopDetect map[*ListHead]bool

	retry := false
	_ = retry
RETRY:
	loopDetect = map[*ListHead]bool{}
	for cur := l.Front(); !cur.Empty(); cur = cur.Next() {
	EACH_RETRY:
		if loopDetect[cur] {
			fmt.Printf("loop")
			retry = true
			goto RETRY
		}
		loopDetect[cur] = true
		if uintptr(unsafe.Pointer(cur.next))&1 > 0 {
			loopDetect[cur] = false
			goto EACH_RETRY
		}

		cnt++
	}
	return
}

func (l *ListHead) len() (cnt int) {

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
	if MODE_CONCURRENT {
		return l.frontCc()
	}

	return l.front()
}

func (l *ListHead) front() (head *ListHead) {
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

func (l *ListHead) frontCc() (head *ListHead) {

	defer func() {

		if head.prev == head && head.next != head {
			fmt.Printf("start terminate? head.next.Empty()=%v\n", head.next.Empty())
			return
		}
		if head.prev != head && head.next == head {
			fmt.Printf("end terminate?")
			return
		}

	}()

	start := l

	if start.IsPurged() {
		start = start.Prev()
		//start = start.ActiveList()
	}
	if start.Empty() && !start.Next().Empty() {
		start = start.Next()
	}
	if start.Empty() && !start.Prev().Empty() {
		start = start.Prev()
	}

	isInfinit := map[*ListHead]bool{}
	for head = start; !head.Prev().Empty(); head = head.Prev() {
		if head.IsFirst() {
			return head
		}
		if isInfinit[head] {
			panic("infinit loop")
		}
		isInfinit[head] = true
	}
	//panic("front not found")
	//head = head.prepareFirst(false)
	return
}

func (l *ListHead) Back() (head *ListHead) {
	if MODE_CONCURRENT {
		return l.backCc()
	}
	return l.back()

}
func (l *ListHead) backCc() (head *ListHead) {
	isInfinit := map[*ListHead]bool{}

	start := l

	if start.IsPurged() {
		start = start.Prev()
		//start = start.ActiveList()
	}
	if start.Empty() && !start.Next().Empty() {
		start = start.Next()
	}
	if start.Empty() && !start.Prev().Empty() {
		start = start.Prev()
	}

	for head = l; !head.Next().Empty(); head = head.Next() {
		if head.IsLast() {
			return head
		}
		if isInfinit[head] {
			panic("infinit loop")
		}
		isInfinit[head] = true
	}
	return
}

func (l *ListHead) back() (head *ListHead) {
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

	start := head
RETRY:

	hasCur := map[*ListHead]int{}
	hasNext := map[*ListHead]int{}
	cnt := 0

	for cur, next := start, start.Next(); !next.IsLast(); cur, next = cur.Next(), next.Next() {
		if _, ok := hasCur[cur]; ok {
			return fmt.Errorf("this list is partial loop cnt=%d hasCur[cur]=%d", cnt, hasCur[cur])
		}
		if _, ok := hasNext[next]; ok {
			fmt.Printf("this list is partial loop cur=%p next=%p\n", cur, next)
			return fmt.Errorf("this list is partial loop cnt=%d hasNext[next]=%d", cnt, hasNext[next])
		}
		hasCur[cur] = cnt
		hasNext[next] = cnt

		if cur.isMarkedForDeleteWithoutError() {
			start = cur.Prev()
			goto RETRY
		}
		if next.isMarkedForDeleteWithoutError() {
			start = cur.Prev()
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

func (head *ListHead) IsPurged() bool {
	if !head.isMarkedForDeleteWithoutError() {
		return false
	}

	return head.Prev().Next() != head

}

func (head *ListHead) Purge() (active *ListHead, purged *ListHead) {

	if !head.canPurge() {
		return head, nil
	}
	err := head.MarkForDelete()
	if err != nil {
		return head, nil
	}
	isSafety, _ := head.IsSafety()
	if isSafety {
		head.Init()
	} else {
		_ = isSafety
	}
	return nil, head
}

func (head *ListHead) AvoidNotAppend(err error) *ListHead {
	switch err {
	case ErrNotAppend:
		if head.Prev().isMarkedForDeleteWithoutError() {
			return head.Prev().AvoidNotAppend(ErrMarked)
		}
		return head.Prev()
	case ErrMarked:
		if head.Prev() == head {
			return head
		}
		if head.Prev().isMarkedForDeleteWithoutError() {
			return head.Prev().AvoidNotAppend(err)
		}
		//TODO: other error pattern
	}
	return head
}

func (head *ListHead) ActiveList() *ListHead {
	if !head.IsPurged() {
		return head
	}
	var active, purged *ListHead

	active = head.Prev().Next()
	purged = head
	purged.Init()
	return active

}

func (head *ListHead) canPurge() bool {

	if head.prev == head {
		return false
	}

	if head.next == head {
		return false
	}
	return true
}

func (head *ListHead) canAdd() bool {

	if head.next == head {
		return false
	}
	return true
}

func Retry(cnt int, fn func(retry int) (done bool, err error)) error {
	return retry(cnt, fn)
}

func retry(cnt int, fn func(retry int) (done bool, err error)) error {

	stats := map[error]int{}
	for i := 0; i < cnt; i++ {
		if done, err := fn(i); done {
			return err
		} else if _, found := stats[err]; !found {
			stats[err] = 1
		} else {
			stats[err]++
		}
	}
	return NewError(ErrTOverRetyry, fmt.Errorf("reach retry limit err=%+v", stats))
}

func PrevNoM(oprev *ListHead) *ListHead {

	prev := uintptr(unsafe.Pointer(oprev))
	mask := uintptr(^uint(0)) ^ 1
	if uintptr(prev)&1 == 0 {
		return oprev
	}

	return PrevNoM((*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(oprev)) & mask)).prev)

}

func NextNoM(ocur *ListHead) *ListHead {

	cur := uintptr(unsafe.Pointer(ocur))
	mask := uintptr(^uint(0)) ^ 1
	if uintptr(cur)&1 == 0 {
		return ocur
	}

	return NextNoM((*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(cur)) & mask)).next)
}

func LastNoM(ocur *ListHead) *ListHead {

	if ocur.next == ocur {
		return ocur
	}

	return LastNoM(NextNoM(ocur).next)

}

func findByFn(head *ListHead, traverseFn func(*ListHead) *ListHead, condFn func(src, dst *ListHead) bool) (result *ListHead, nest int) {

	nest = 0
	for cur, next := head, traverseFn(head); !cur.Empty() && !next.Empty(); cur, next = next, traverseFn(next) {
		if condFn(cur, next) {
			return cur, nest
		}
		nest++
	}
	return nil, nest
}

func isFound(r *ListHead, n int) bool {
	return r != nil
}

func (head *ListHead) findNextNoM(exptected *ListHead) (*ListHead, int) {
	return findByFn(head,
		func(c *ListHead) *ListHead {
			if c == NextNoM(c) {
				return c.next
			}
			return NextNoM(c)
		},
		func(src, dst *ListHead) bool {
			return src == exptected
		})
}

func (head *ListHead) findPrevNoM(exptected *ListHead) (*ListHead, int) {
	return findByFn(head,
		func(c *ListHead) *ListHead {
			if c == PrevNoM(c) {
				return c.prev
			}
			return PrevNoM(c)
		},
		func(src, dst *ListHead) bool {
			return src == exptected
		})
}

func (head *ListHead) IsMarked() bool {

	if uintptr(unsafe.Pointer(head.prev))&1 > 0 {
		return true
	}
	if uintptr(unsafe.Pointer(head.next))&1 > 0 {
		return true
	}
	return false
}

func (head *ListHead) IsSafety() (bool, error) {

	prev := PrevNoM(head.prev)
	next := NextNoM(head.next)

	if prev.next.IsMarked() {
		return false, nil
	}

	if next.prev.IsMarked() {
		return false, nil
	}
	return true, nil
}
