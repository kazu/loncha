// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
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
	ErrTNoSafety
	ErrTNoContinous
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
	ErrNoSafetyOnAdd     error = NewError(ErrTNoSafety, errors.New("element is not safety to append"))
	ErrNoContinous       error = NewError(ErrTNoContinous, errors.New("element is not continus"))
	//ErrNoSafety          error = NewError(ErrTNoSafety, errors.New("element is not safety to append"))
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

func (head *ListHead) Ptr() unsafe.Pointer {
	return unsafe.Pointer(head)
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
	start.next = head
	end.prev = head

	// a reader may still load the links of head, as a Get of an item pool
	// does while an expand runs Init on the old pool
	StoreListHead(&head.prev, start)
	StoreListHead(&head.next, end)
}

func (head *ListHead) InitAsEmpty() {

	end := NewEmpty()

	head.prev = head
	head.next = end

	end.next = end
	end.prev = head

}

func InitAsEmpty(head *ListHead, tail *ListHead) {

	head.prev, head.next, tail.prev, tail.next =
		head, tail, head, tail

}

type TraverseType uint32

const (
	TravDirect TraverseType = iota
	TravWaitNoMark
	TravSkipMark
)

type ModeTraverse struct {
	t      TraverseType
	Mu     func(*ListHead) *sync.RWMutex
	e      error
	confMu sync.Mutex
}

var DefaultModeTraverse *ModeTraverse = &ModeTraverse{t: TravDirect}

func (mode ModeTraverse) Error() string {
	if mode.e != nil {
		return mode.e.Error()
	}
	return ""
}

func NewTraverse() *ModeTraverse {
	return &ModeTraverse{t: TravDirect}
}

func (mt *ModeTraverse) SetError(e error) {
	mt.confMu.Lock()
	defer mt.confMu.Unlock()
	mt.e = e
}

//go:nocheckptr
func (mt *ModeTraverse) Type() TraverseType {
	// mt.confMu.RLock()
	// defer mt.confMu.RUnlock()
	return TraverseType(atomic.LoadUint32((*uint32)(&mt.t)))

}

func (mt *ModeTraverse) SetType(t TraverseType) {
	atomic.StoreUint32((*uint32)(&mt.t), uint32(t))
	return
}

func (mt *ModeTraverse) Option(opts ...TravOpt) (prevs []TravOpt) {

	var mu sync.Mutex
	mu.Lock()
	for _, opt := range opts {
		prevs = append(prevs, opt(mt))
	}
	mu.Unlock()
	return
}

type TravOpt func(*ModeTraverse) TravOpt

func Trav(t TraverseType) TravOpt {

	return func(m *ModeTraverse) TravOpt {
		m.confMu.Lock()
		defer m.confMu.Unlock()
		prev := m.Type()
		m.SetType(t)

		return Trav(prev)
	}
}

func WaitNoM() TravOpt {
	return Trav(TravWaitNoMark)
}

func Direct() TravOpt {
	return Trav(TravDirect)
}

func Lock(fn func(*ListHead) *sync.RWMutex) TravOpt {
	return func(m *ModeTraverse) TravOpt {
		prev := m.Mu
		m.Mu = fn
		if prev == nil {
			return nil
		}
		return Lock(prev)
	}
}

func InitAfterSafety(retry int) func(*ListHead) error {

	return func(head *ListHead) error {
		return Retry(retry, func(c int) (exit bool, err error) {
			if ok, _ := head.IsSafety(); !ok {
				return false, ErrNoSafetyOnAdd
			}
			head.Init()
			return true, nil
		})
	}

}

// prevLoad and nextLoad load a link as it is, with the mark bit of a delete.
//
//go:nocheckptr
func prevLoad(head *ListHead) (prev *ListHead) {
	prev = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))
	return prev
}

// prevDirect returns the node head.prev leads to, without the mark bit.
func prevDirect(head *ListHead) (prev *ListHead) {
	return nodePrev(head)
}

func prevWaitNoMark(head *ListHead) (prev *ListHead) {
	var err error
	stepAt("prev.waitNoMark", head, nil, nil)
	prev = nodePrev(head)
	for retry := 100; retry > 0; retry-- {
		if !prev.IsMarked() {
			err = nil
			break
		}
		err = ErrMarked
		prev = nodePrev(head)
	}

	if err != nil {
		DefaultModeTraverse.e = err
		return nil
	}

	return prev
}

func prevSkipMark(head *ListHead) (prev *ListHead) {
	var err error
	prev = nodePrev(head)
	for retry := 100; retry > 0; retry-- {
		if !prev.IsMarked() {
			err = nil
			break
		}
		prev = PrevNoM(prevLoad(head))
		err = nil
		break
	}

	if err != nil {
		DefaultModeTraverse.e = err
		return nil
	}

	return prev
}

// traverseType returns the traverse type of one Prev or Next call: the type
// of DefaultModeTraverse changed by opts. opts apply to a copy, so that
// calls running at the same time do not take the options of each other.
func traverseType(opts []TravOpt) TraverseType {
	if len(opts) == 0 {
		return DefaultModeTraverse.Type()
	}
	mode := ModeTraverse{t: DefaultModeTraverse.Type()}
	for _, opt := range opts {
		opt(&mode)
	}
	return mode.Type()
}

func (head *ListHead) Prev(opts ...TravOpt) *ListHead {
	//return ListPrev(head, opts...)
	return ListPrev(head, opts...)
}

func ListPrev(head *ListHead, opts ...TravOpt) (prev *ListHead) {
	// if len(opts) == 0 && DefaultModeTraverse.prev != nil {
	// 	return DefaultModeTraverse.prev(head)
	// }
	// return prevDefault(head, opts...)

	t := DefaultModeTraverse.Type()
	if len(opts) > 0 {
		t = traverseType(opts)
	}
	switch t {
	case TravDirect:
		return prevDirect(head)
	case TravWaitNoMark:
		return prevWaitNoMark(head)
	case TravSkipMark:
		return prevSkipMark(head)
	}

	return prevDefault(head)
}

func prevDefault(head *ListHead, opts ...TravOpt) (prev *ListHead) {

	//mode := ModeTraverse{t: TravDirect}
	mode := DefaultModeTraverse

	if len(opts) > 0 {
		mode.Option(opts...)
	}

	if DefaultModeTraverse.Mu != nil && !head.Empty() {
		DefaultModeTraverse.Mu(head).RLock()
		defer DefaultModeTraverse.Mu(head).RUnlock()
	}

	var err error
	var exit bool
	_ = exit

	for retry := 100; retry > 0; retry-- {
		prev = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))

		// defer func() {
		// 	exit, err = head.rewriteResultOnPrev(mode, prev, exit, err)
		// }()
		if DefaultModeTraverse.t == TravDirect {
			exit = true
			break
		}

		if !prev.IsMarked() {
			exit = true
			break
		}
		switch DefaultModeTraverse.t {
		case TravDirect:
			exit = true
			break
		case TravWaitNoMark:
			exit, err = false, ErrMarked
			continue
		case TravSkipMark:
			prev = PrevNoM(prev)
			exit = true
			break
		}
		// defailt is  TravDirect

		exit = true
		return
	}

	if err != nil {
		// FIXME: log warning
		DefaultModeTraverse.e = err
	}

	return prev
}

func (head *ListHead) DirectNext() *ListHead {
	return nextLoad(head)
}

func (head *ListHead) PtrNext() **ListHead {
	//return atomic.LoadPointer(&head.next)
	return &head.next
}

func (head *ListHead) DirectPrev() *ListHead {
	return prevLoad(head)
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

func (head *ListHead) rewriteResultOnNext(mode ModeTraverse, next *ListHead, oexit bool, oerr error) (exit bool, err error) {
	if !oexit {
		return oexit, oerr
	}
	if mode.t == TravSkipMark {
		if PrevNoM(next.prev) != head {
			return false, NewError(ErrTNoContinous, errors.New("element is not continus on SkipMark"))
		}
	}

	if next.prev != head && !head.Empty() && !next.Empty() {
		return false, ErrNoContinous
	}

	return oexit, oerr
}

func (head *ListHead) rewriteResultOnPrev(mode ModeTraverse, prev *ListHead, oexit bool, oerr error) (exit bool, err error) {
	if !oexit {
		return oexit, oerr
	}
	if mode.t == TravSkipMark {
		if NextNoM(prev.prev) != head {
			return false, NewError(ErrTNoContinous, errors.New("element is not continus on SkipMark"))
		}
	}

	if prev.next != head && !head.Empty() && !prev.Empty() {
		return false, ErrNoContinous
	}

	return oexit, oerr
}

// nextDirect returns the node head.next leads to, without the mark bit that
// a delete of head puts on the link, as nextDirect of elist_head does.
func nextDirect(head *ListHead) (next *ListHead) {
	return nodeNext(head)
}

//go:nocheckptr
func nextLoad(head *ListHead) (next *ListHead) {
	next = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next))))
	return next
}

func nextWaitNoMark(head *ListHead) (next *ListHead) {
	var err error
	next = nodeNext(head)
	for retry := 100; retry > 0; retry-- {
		if !next.IsMarked() {
			err = nil
			break
		}
		err = ErrMarked
		next = nodeNext(head)
	}

	if err != nil {
		DefaultModeTraverse.e = err
		return nil
	}

	return next
}

func nextSkipMark(head *ListHead) (next *ListHead) {
	var err error
	next = nodeNext(head)
	for retry := 100; retry > 0; retry-- {
		if !next.IsMarked() {
			err = nil
			break
		}
		next = NextNoM(nextLoad(head))
		err = nil
		break
	}

	if err != nil {
		DefaultModeTraverse.e = err
		return nil
	}

	return next
}

func (head *ListHead) Next(opts ...TravOpt) *ListHead {
	//return ListPrev(head, opts...)
	return ListNext(head, opts...)
}

func ListNext(head *ListHead, opts ...TravOpt) *ListHead {
	t := DefaultModeTraverse.Type()
	if len(opts) > 0 {
		t = traverseType(opts)
	}
	switch t {
	case TravDirect:
		return nextDirect(head)
	case TravWaitNoMark:
		return nextWaitNoMark(head)
	case TravSkipMark:
		return nextSkipMark(head)
	}

	return nextDefault(head)
}

func nextDefault(head *ListHead, opts ...TravOpt) (next *ListHead) {
	//MENTION: ignore error. should use NextWithError()

	if DefaultModeTraverse.Mu != nil && !head.Empty() {
		DefaultModeTraverse.Mu(head).RLock()
		defer DefaultModeTraverse.Mu(head).RUnlock()
	}
	var err error
	var exit bool
	_ = exit

	for retry := 100; retry > 0; retry-- {

		next = head.DirectNext()

		// defer func() {
		// 	exit, err = head.rewriteResultOnNext(*mode, nextElement, exit, err)
		// }()
		if DefaultModeTraverse.t == TravDirect {
			exit = true
			break
		}

		if !next.IsMarked() {
			exit = true
			break
		}
		switch DefaultModeTraverse.t {
		case TravDirect:
			exit = true
			break
		case TravWaitNoMark:
			exit, err = false, ErrNextMarked
			continue
		case TravSkipMark:
			next = NextNoM(next)
			exit = true
			break
		}
		next = head.NextWithError().head
		exit = true
		break
	}

	if err != nil {
		//FIXME: log warning
		DefaultModeTraverse.e = err
	}
	return
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
			//MENTION: if not called, remove this codes
			AddRecoverState("nextElement.prev != head")
			//nextElement.recoverPrev(head)
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

// test for mutex mode
type mutex struct {
	sync.Mutex
	enable bool
}

func newMutex(t bool) *mutex {
	return &mutex{enable: t}
}

func (mu *mutex) Lock() {
	if !mu.enable {
		return
	}
	mu.Mutex.Lock()
}

func (mu *mutex) Unlock() {
	if !mu.enable {
		return
	}
	mu.Mutex.Unlock()
}

var mu4Add *mutex = newMutex(false)

//  prev ---------------> next
//        \--> new --/
//   prev --> next     prev ---> new
func listAddWitCas(new, prev, next *ListHead, fn func(*ListHead) *sync.RWMutex) (err error) {
	// backup for roolback
	oNewPrev := new.prev
	oNewNext := new.next
	if fn != nil {
		if !prev.Empty() {
			fn(prev).Lock()
			defer fn(prev).Unlock()
		}
		if !next.Empty() {
			fn(next).Lock()
			defer fn(next).Unlock()
		}
	}
	rollback := func(new *ListHead) {
		StoreListHead(&new.prev, oNewPrev)
		StoreListHead(&new.next, oNewNext)
	}
	_ = rollback

	// new.prev -> prev, new.next -> next
	StoreListHead(&new.prev, prev)
	StoreListHead(&new.next, next)

	mu4Add.Lock()
	defer mu4Add.Unlock()
	stepAt("add.cas1", new, prev, next)
	// an insert before next waits while next is half inserted: the node after
	// next does not link back to next yet
	if nn := nodeNext(next); nn != next && nodePrev(nn) != next {
		goto ROLLBACK
	}
	if !Cas(&prev.next, next, new) {
		goto ROLLBACK
	}
	stepAt("add.cas2", new, prev, next)
	if !Cas(&next.prev, prev, new) {

		stepAt("add.rollback", new, prev, next)
		// take new out as a delete of new does, so that a delete of next
		// that passed over the link from prev to new sees it removed. new
		// keeps its marked links like any deleted node, so that the nodes
		// next to it are not taken as safe to reuse through it
		if err := new.MarkForDelete(); err != nil {
			return err
		}
		return NewError(ErrTCasConflictOnAdd,
			fmt.Errorf("listAddWithCas() please retry: new=%s prev=%s next=%s", new.P(), prev.P(), next.P()))
	}

	return nil

ROLLBACK:

	rollback(new)
	return NewError(ErrTCasConflictOnAdd,
		fmt.Errorf("listAddWithCas() please retry: new=%s prev=%s next=%s", new.P(), prev.P(), next.P()))

}

//go:nocheckptr
func (l *ListHead) WithOutMark() *ListHead {
	const mask = uintptr(^uint(0)) ^ 1

	return (*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(l)) & mask))

}

func (l *ListHead) MarkForDelete(opts ...TravOpt) (err error) {

	mode := ModeTraverse{t: TravDirect}
	defer mode.Error()
	for _, opt := range opts {
		opt(&mode)
	}

	if !l.canPurge() {
		return ErrNotMarked
	}
	stepAt("del.purgeable", l, nil, nil)
	mu4Add.Lock()
	defer mu4Add.Unlock()

	var (
		ErrDeketeStep0 error = errors.New("fail step 0")
		ErrDeketeStep1 error = errors.New("fail step 1")
		ErrDeketeStep2 error = errors.New("fail step 2")
		ErrDeketeStep3 error = errors.New("fail step 3")
	)
	_, _ = ErrDeketeStep2, ErrDeketeStep3

	try := func(retry int) (fin bool, err error) {
		prev1 := prevLoad(l).WithOutMark()
		next1 := nextLoad(l).WithOutMark()
		stepAt("del.begin", l, prev1, next1)
		//prev1 := (*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(l.prev)) & mask))
		//next1 := (*ListHead)(unsafe.Pointer(uintptr(unsafe.Pointer(l.next)) & mask))
		if mode.Mu != nil {
			if !prev1.Empty() {
				mode.Mu(prev1).Lock()
				defer mode.Mu(prev1).Unlock()
			}
			mode.Mu(l).Lock()
			defer mode.Mu(l).Unlock()
			if !next1.Empty() {
				mode.Mu(next1).Lock()
				defer mode.Mu(next1).Unlock()
			}
		}

		prev := prev1
		next := next1

		if retry > 0 {
			// a neighbor is in the middle of an insert or a delete
			runtime.Gosched()
		}

		if !MarkListHead(&l.next, next) {
			AddRecoverState("remove: retry marked next")
			return false, ErrDeketeStep0
		}
		stepAt("del.nextMarked", l, prev1, next1)
		if !MarkListHead(&l.prev, prev) {
			AddRecoverState("remove: retry marked prev")
			return false, ErrDeketeStep1
		}
		stepAt("del.marked", l, prev1, next1)
		if !prev1.Empty() && mode.Mu != nil {
			mode.Mu(prev1).Lock()
			defer mode.Mu(prev1).Unlock()
		}
		prev2 := PrevNoM(prevLoad(l))
		next2 := NextNoM(nextLoad(l))
		if mode.Mu != nil {
			if !prev2.Empty() {
				mode.Mu(prev2).Lock()
				defer mode.Mu(prev2).Unlock()
			}
			if !next2.Empty() {
				mode.Mu(next2).Lock()
				defer mode.Mu(next2).Unlock()
			}
		}

		_, _ = prev2, next2

		// relink the links to l from the nearest nodes that are not marked,
		// passing the marked nodes between them and l. An insert that has
		// made only its first CAS next to l is waited for: it either
		// finishes or removes its node.
		if halfInsertedBefore(l) {
			return false, ErrDeketeStep2
		}
		if x, v := linkingPrev(l); x != nil && uintptr(unsafe.Pointer(v))&1 == 0 {
			next := NextNoM(nextLoad(l))
			stepAt("del.relinkNext", l, nil, next)
			Cas(&x.next, v, next)
		}
		if halfInsertedAfter(l) {
			return false, ErrDeketeStep2
		}
		if z, v := linkingNext(l); z != nil && uintptr(unsafe.Pointer(v))&1 == 0 {
			Cas(&z.prev, v, linkedBefore(l, z))
		}
		stepAt("del.check", l, prev1, next1)
		if !l.unlinked() {
			AddRecoverState("remove: found node to me")
			return false, ErrDeketeStep2
		}
		return true, nil
	}
	for retry := 0; ; retry++ {
		if fin, e := try(retry); fin {
			err = e
			break
		}
	}

	if err != nil {
		mode.e = err
	}

	return err
}

type checkOpt struct {
	isMarkedFalse bool
}

func newCheckOpt() *checkOpt {
	return &checkOpt{isMarkedFalse: true}
}

type OptMark func(*checkOpt)

func Marked(t bool) OptMark {

	return func(c *checkOpt) {
		c.isMarkedFalse = !t
	}
}

func (l *ListHead) checkPrevNext(opts ...OptMark) bool {
	conf := newCheckOpt()
	for _, opt := range opts {
		opt(conf)
	}

	if conf.isMarkedFalse && l.IsMarked() {
		return false
	}
	if conf.isMarkedFalse && l.prev.IsMarked() {
		return false
	}
	if conf.isMarkedFalse && l.next.IsMarked() {
		return false
	}

	if l.prev == l && l.next.prev == l {
		return true
	}
	if l.next == l && l.prev.next == l {
		return true
	}

	if l.prev.next != l || l.next.prev != l {
		return false
	}

	return true

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
		return prevLoad(l) == l || nextLoad(l) == l
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

// frontPrev returns the nearest node before head that is not marked, for the
// walk of frontCc.
func frontPrev(head *ListHead) *ListHead {
	stepAt("front.prev", head, nil, nil)
	return PrevNoM(prevLoad(head))
}

func (l *ListHead) frontCc() (head *ListHead) {

	defer func() {
		retryed := false
	RETRY:
		if prevLoad(head) == head && nextLoad(head) != head {
			if retryed {
				// FIXME: log warning
				//fmt.Printf("start terminate? head.next.Empty()=%v\n", head.next.Empty())
				_ = "head empty"
			}
			head = nextLoad(head)
			retryed = true
			goto RETRY
		}
		if prevLoad(head) != head && nextLoad(head) == head {
			_ = "end terminate?"
			return
		}

	}()

	start := l

	if start.IsPurged() {
		start = start.Prev(WaitNoM())
	}
	if start.Empty() && !start.Next(WaitNoM()).Empty() {
		start = start.Next()
	}
	if start.Empty() && !start.Prev(WaitNoM()).Empty() {
		start = start.Prev()
	}
	retry := 2

	cnt := -1
	var next *ListHead
	_ = next

	isInfinit := map[*ListHead]int{}
	// walk over the nodes being deleted instead of waiting for them, which
	// gives up with nil after 100 reads
	for head, next = start, start; !frontPrev(head).Empty(); next, head = head, frontPrev(head) {
		if head.IsMarked() {
			retry = 2
			head = next
			goto RETRYMARKED
		}
		if head.IsFirst() {
			return head
		}
		if _, ok := isInfinit[head]; ok {
			if retry < 1 {
				// FIXME: log warning
				_ = "infinit loop"
			}
			goto RETRY
		}
		isInfinit[head] = cnt
		continue
	RETRY:
		head = l
		isInfinit = map[*ListHead]int{}
	RETRYMARKED:
		cnt = -1
		retry--
	}
	return
}

func (l *ListHead) Back() (head *ListHead) {
	if MODE_CONCURRENT {
		return l.backCc()
	}
	return l.back()

}
func (l *ListHead) backCc() (head *ListHead) {
	isInfinit := map[*ListHead]int{}

	start := l

	if start.IsPurged() {
		start = start.Prev(WaitNoM())
		//start = start.ActiveList()
	}
	if start.Empty() && !start.Next(WaitNoM()).Empty() {
		start = start.Next()
	}
	if start.Empty() && !start.Prev(WaitNoM()).Empty() {
		start = start.Prev()
	}

	retry := 2

	cnt := -1
	var prev *ListHead
	_ = prev
	retryInfinited := map[*ListHead]bool{}

	for head, prev = l, head; !head.Next(WaitNoM()).Empty(); prev, head = head, head.Next(WaitNoM()) {
		cnt++
		if head.IsMarked() {
			retry = 2
			head = prev
			if head.Empty() || head.IsSingle() {
				prevIsSIngle := prev.IsSingle()
				_ = prevIsSIngle
				_ = "lost empty node"
			}
			goto RETRYMARKED
		}
		if head.IsLast() {
			return head
		}
		if _, ok := isInfinit[head]; ok {
			if retry < 1 {
				if retryInfinited[head] {
					// FIXME: log warning
					_ = "infinit loop"
				}
				retry = 2
				retryInfinited[head] = true
			}
			goto RETRY
		}
		isInfinit[head] = cnt
		continue
	RETRY:
		head = l
		isInfinit = map[*ListHead]int{}
	RETRYMARKED:
		cnt = -1
		retry--
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

	stepAt("cursor.next", cur.Pos, nil, nil)
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

func (head *ListHead) Purge(opts ...func(*ListHead) error) (active *ListHead, purged *ListHead) {

	if !head.canPurge() {
		return head, nil
	}
	err := head.MarkForDelete()
	if err != nil {
		return head, nil
	}
	mu4Add.Lock()
	defer mu4Add.Unlock()
	if len(opts) == 0 {
		opts = append(opts, InitAfterSafety(100))
	}

	for _, opt := range opts {
		if opt(head) != nil {
			break
		}
	}

	return nil, head

}

func (head *ListHead) AvoidNotAppend(err error) *ListHead {
	switch err {
	case ErrNotAppend:
		if nodePrev(head).isMarkedForDeleteWithoutError() {
			return nodePrev(head).AvoidNotAppend(ErrMarked)
		}
		return nodePrev(head)
	case ErrMarked:
		if nodePrev(head) == head {
			return head
		}
		if nodePrev(head).isMarkedForDeleteWithoutError() {
			return nodePrev(head).AvoidNotAppend(err)
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

//go:nocheckptr
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

// PrevNoM returns the nearest node that is not marked, starting at the node
// oprev leads to and walking backward over the marked nodes. The mark bit of
// oprev itself belongs to the node holding the link, not to the node it
// leads to.
//
//go:nocheckptr
func PrevNoM(oprev *ListHead) *ListHead {

	prev := oprev.WithOutMark()
	if nodePrev(prev) == prev || !prev.IsMarked() {
		return prev
	}
	return PrevNoM(prevLoad(prev))

}

// NextNoM returns the nearest node that is not marked, starting at the node
// ocur leads to and walking forward over the marked nodes.
//
//go:nocheckptr
func NextNoM(ocur *ListHead) *ListHead {

	next := ocur.WithOutMark()
	if nodeNext(next) == next || !next.IsMarked() {
		return next
	}
	return NextNoM(nextLoad(next))
}

// nodePrev and nodeNext load a link of head and drop its mark bit.
//
//go:nocheckptr
func nodePrev(head *ListHead) *ListHead {
	return prevLoad(head).WithOutMark()
}

//go:nocheckptr
func nodeNext(head *ListHead) *ListHead {
	return nextLoad(head).WithOutMark()
}

// linkingPrev returns the nearest node before head that is not marked and
// the value of its next, when that next leads to head passing only marked
// nodes. It returns nil when no such link is left.
//
//go:nocheckptr
func linkingPrev(head *ListHead) (*ListHead, *ListHead) {
	x := PrevNoM(prevLoad(head))
	v := nextLoad(x)
	for cur := nodeNext(x); cur != x; cur = nodeNext(cur) {
		if cur == head {
			return x, v
		}
		if !cur.IsMarked() || nodeNext(cur) == cur {
			break
		}
	}
	return nil, nil
}

// linkingNext returns the nearest node after head that is not marked and
// the value of its prev, when that prev leads to head passing only marked
// nodes. It returns nil when no such link is left.
//
//go:nocheckptr
func linkingNext(head *ListHead) (*ListHead, *ListHead) {
	z := NextNoM(nextLoad(head))
	v := prevLoad(z)
	for cur := nodePrev(z); cur != z; cur = nodePrev(cur) {
		if cur == head {
			return z, v
		}
		if !cur.IsMarked() || nodePrev(cur) == cur {
			break
		}
	}
	return nil, nil
}

// halfInsertedBefore reports whether a node that is not marked links to
// head by its next while the node before it does not link to head: an
// insert before head has made its first CAS and not its second.
//
//go:nocheckptr
func halfInsertedBefore(head *ListHead) bool {
	x := PrevNoM(prevLoad(head))
	for cur := nodeNext(x); cur != x && cur != head; cur = nodeNext(cur) {
		if nodeNext(cur) == cur {
			return false
		}
		if !cur.IsMarked() {
			return nodeNext(cur) == head
		}
	}
	return false
}

// halfInsertedAfter reports whether the nearest node after head that is not
// marked is linked from head while the node after it still links back to
// head: an insert after head has made its first CAS and not its second.
//
//go:nocheckptr
func halfInsertedAfter(head *ListHead) bool {
	z := NextNoM(nextLoad(head))
	if z == head || nodeNext(z) == z {
		return false
	}
	return nodePrev(nodeNext(z)) == head
}

// linkedBefore returns the node whose next is z, walking forward from the
// nearest node before head that is not marked, or that node when the walk
// does not reach z.
//
//go:nocheckptr
func linkedBefore(head, z *ListHead) *ListHead {
	x := PrevNoM(prevLoad(head))
	for cur := x; nodeNext(cur) != cur; cur = nodeNext(cur) {
		if nodeNext(cur) == z {
			return cur
		}
		if nodeNext(cur) == x {
			break
		}
	}
	return x
}

// unlinked reports whether no node that is not marked links to head any
// more, and no insert next to head is between its two CASes.
func (head *ListHead) unlinked() bool {
	if x, _ := linkingPrev(head); x != nil {
		return false
	}
	if z, _ := linkingNext(head); z != nil {
		return false
	}
	return !halfInsertedBefore(head) && !halfInsertedAfter(head)
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

	if uintptr(unsafe.Pointer(prevLoad(head)))&1 > 0 {
		return true
	}
	if uintptr(unsafe.Pointer(nextLoad(head)))&1 > 0 {
		return true
	}
	return false
}

func (head *ListHead) IsSafety() (bool, error) {

	prev := PrevNoM(prevLoad(head))
	next := NextNoM(nextLoad(head))
	stepAt("safety.nodes", head, prev, next)

	if nodeNext(prev).IsMarked() {
		return false, nil
	}

	if nodePrev(next).IsMarked() {
		return false, nil
	}
	if nodeNext(prev) == head {
		return false, nil
	}
	if nodePrev(next) == head {
		return false, nil
	}
	if !head.unlinked() {
		return false, nil
	}

	return true, nil
}

func (head *ListHead) CountTo(dst *ListHead) (cnt int) {

	cnt = 0

	for cur := head; !head.Empty(); cur = cur.Next(WaitNoM()) {

		if cur == dst {
			return
		}
		cnt++
	}

	return cnt

}
