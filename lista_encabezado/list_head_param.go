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
	"sync"
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

func toNode(head *ListHead) *ListHead {
	if head.prev == head {
		return head.Next()
	}
	if head.next == head {
		return head.Prev()
	}
	return head

}

func (head *ListHead) prepareFirst(useTerminater bool) *ListHead {

	prev := head.prev

	if !head.IsFirst() {
		return head
	}
	if !head.isMarkedForDeleteWithoutError() {
		if head.next == head {
			goto ENSURE
		}
		return head
	}

ENSURE:
	if useTerminater {
		return prev
	}
	return prev.Next()

}

func (head *ListHead) AppendWithRecover(new *ListHead) (nHead *ListHead, err error) {

	start := head
	if head.isMarkedForDeleteWithoutError() {
		return new, ErrMarked
	}

	if start.next == start {
		start = start.Prev()
	}

	for elm := start; true; elm = elm.Prev() {
		nHead, err := elm.Append(new)
		if err == nil {
			return nHead, err
		}
		if elm.Prev() == elm {
			break
		}
	}

	return new, err

}

func (head *ListHead) Append(new *ListHead) (*ListHead, error) {

	if new.IsMarked() {
		initAfterSafety(new)
	}

	nlast, err := head.append(new)
	if err == nil {
		return nlast, err
	}
	nlast2 := nlast.AvoidNotAppend(err)
	return nlast2.append(new)
}

func (head *ListHead) InsertBefore(new *ListHead, opts ...TravOpt) (*ListHead, error) {

	if new.IsMarked() {
		initAfterSafety(new)
	}

	// nlast, err := head.append(new)
	// if err == nil {
	// 	return nlast, err
	// }
	// nlast2 := nlast.AvoidNotAppend(err)
	// return nlast2.append(new)
	if !MODE_CONCURRENT {
		head.add(new)
		return new, nil
	}
	if head.isMarkedForDeleteWithoutError() {
		return head, ErrMarked
	}
	stepAt("insert.begin", new, nil, head)

	// if head.prev.isMarkedForDeleteWithoutError() {
	// 	return head, ErrMarked
	// }
	// if !head.prev.canAdd() {
	// 	return head, ErrNotAppend
	// }
	nNode := toNode(new)
	if err := head.insertBefore(nNode, opts...); err != nil {
		return head, err
	}
	return head, nil

}

// TryInsertBefore links new just before head in one attempt, as
// TryInsertBefore of elist_head does: it reads the node before head once and
// links new there only if accept returns true for that node. It returns an
// error without linking new when head is marked, is not linked to a previous
// node, is rejected by accept, or when another goroutine changed the links
// first; the caller finds the position again.
func (head *ListHead) TryInsertBefore(new *ListHead, accept func(prev *ListHead) bool) error {

	if new.IsMarked() {
		initAfterSafety(new)
	}
	if head.isMarkedForDeleteWithoutError() {
		return ErrMarked
	}
	stepAt("insert.begin", new, nil, head)

	prev := prevLoad(head)
	// the mark on head.prev belongs to a delete of head
	if uintptr(unsafe.Pointer(prev))&1 != 0 {
		return ErrMarked
	}
	if prev == head || !accept(prev) {
		return ErrNotAppend
	}
	return listAddWitCas(toNode(new), prev, head, nil)
}

func (head *ListHead) append(new *ListHead) (*ListHead, error) {
	if !MODE_CONCURRENT {
		head.add(new)
		return new, nil
	}
	if head.isMarkedForDeleteWithoutError() {
		return head, ErrMarked
	}

	if !head.canAdd() {
		return head, ErrNotAppend
	}

	nNode := toNode(new)
	if err := head.add(nNode); err != nil {
		return head, err
	}

	return head, nil
}

func (head *ListHead) Add(new *ListHead) *ListHead {
	_, err := head.Append(new)
	if err != nil {
		return nil
	}
	return new
}

func (head *ListHead) IsSingle() bool {
	if head.Prev() == nil || !head.Prev().Empty() {
		return false
	}

	if head.Next() == nil || !head.Next().Empty() {
		return false
	}
	return true

}

func (head *ListHead) isNext(next *ListHead) bool {

	if head.next != next {
		return false
	}
	if next.prev != head {
		return false
	}

	if head.Next() != next {
		return false
	}
	if next.Prev() != head {
		return false
	}

	return true

}

// retryUntilDone calls fn until it reports done, yielding between the tries.
// A CAS of an insert fails only when another insert or delete changed the
// links first, so the list as a whole moves on while one insert retries.
func retryUntilDone(fn func(retry int) (done bool, err error)) error {
	for r := 0; ; r++ {
		if r > 0 {
			runtime.Gosched()
		}
		if done, err := fn(r); done {
			return err
		}
	}
}

// initAfterSafety waits until no node of the list links to the deleted node
// n any more, and Inits n for another insert. A delete next to n may still
// link to it for a while after n is deleted; this waits for it as a delete
// waits for an insert.
func initAfterSafety(n *ListHead) {
	retryUntilDone(func(retry int) (bool, error) {
		ok, _ := n.IsSafety()
		return ok, nil
	})
	n.Init()
}

func (head *ListHead) add(new *ListHead, opts ...TravOpt) error {
	if MODE_CONCURRENT {
		//retry := 0
		var err error
		mode := ModeTraverse{t: TravDirect}
		for _, opt := range opts {
			opt(&mode)
		}
		if !new.IsSingle() {
			fmt.Printf("Warn:list_head  insert element must be single node\n")
		}
		prev := head
		next := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next))))
		err = retryUntilDone(func(retry int) (finish bool, err error) {
			// the mark on head.next belongs to a delete of head
			if uintptr(unsafe.Pointer(next))&1 != 0 {
				return true, ErrMarked
			}
			if next == head {
				return true, ErrNotAppend
			}
			// a try rolled back by a delete next to it leaves new deleted
			if new.IsMarked() {
				initAfterSafety(new)
			}
			err = listAddWitCas(new,
				prev,
				next, mode.Mu)
			if err == nil {
				return true, err
			}

			next = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&prev.next))))
			AddRecoverState("cas retry")
			return false, err
		})

		return err
	}
	listAdd(new, head, head.next)
	return nil
}

func (head *ListHead) insertBefore(new *ListHead, opts ...TravOpt) error {
	if MODE_CONCURRENT {
		//retry := 0
		var err error
		mode := ModeTraverse{t: TravDirect}
		defer mode.Error()
		for _, opt := range opts {
			opt(&mode)
		}

		if !new.IsSingle() {
			fmt.Printf("Warn: list_head insert element must be single node\n")
		}

		next := head
		prev := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))
		err = retryUntilDone(func(retry int) (finish bool, err error) {
			// the mark on head.prev belongs to a delete of head
			if uintptr(unsafe.Pointer(prev))&1 != 0 || head.isMarkedForDeleteWithoutError() {
				return true, ErrMarked
			}
			if prev == head {
				return true, ErrNotAppend
			}
			// a try rolled back by a delete next to it leaves new deleted
			if new.IsMarked() {
				initAfterSafety(new)
			}
			err = listAddWitCas(new,
				prev,
				next, mode.Mu)
			if err == nil {
				return true, err
			}

			prev = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))
			AddRecoverState("cas retry")
			return false, err
		})

		if err != nil {
			//fmt.Printf("insertBefore(): over retry retry=%d err=%s\n", 100, err.Error())
			mode.e = err
		}

		return err
	}
	listAdd(new, head.prev, head)
	return nil
}

func (head *ListHead) Join(new *ListHead) {
	if new.IsSingle() {
		head.Append(new)
		return
	}
	if !head.canAdd() {
		fmt.Printf("Join(): not appednable node")
		return
	}
	list := head
	var err error
	for cur, next := new, new.Next(); !cur.IsLast(); cur, next = next, next.Next() {
		_, cur := cur.Purge()
		list, err = list.Append(cur)
		if err != nil {
			fmt.Printf("Join(): fail to append err=%s\n", err.Error())
			return
		}
		if next.IsLast() {
			_, next := next.Purge()
			list, err = list.Append(next)
			if err != nil {
				fmt.Printf("Join(): fail to append err=%s\n", err.Error())
			}
			break
		}
	}

}

func (head *ListHead) DeleteElementWithCas(pList List) (err error) {
	return head.DeleteWithCas(pList.PtrListHead())
}

func (head *ListHead) DeleteWithCas(prev *ListHead) (err error) {
	return head.deleteWithCas(prev)
}

func (head *ListHead) deleteFirst() (err error) {

	if head.Next() == head {
		return nil
	}

	if head.Next().IsLast() {
		head.Next().Delete()
		return nil
	}

	next := head.Next()
	nextNext := next.Next()

	next.Delete()
	next.Init()
	next.Join(nextNext)
	head.Init()
	return nil

}

func (head *ListHead) deleteWithCas(prev *ListHead) (err error) {
	use_mark := true

	defer func() {
		if err == nil {
			//if ContainOf(head, l) {
			//	panic("????!!!")
			//}
		}
	}()

	if use_mark {
		err = head.MarkForDelete() // FIXME: race condition 79
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("must support marking")

}

//func ContainOf(head, elm *ListHead) bool {
func ElementIsContainOf(hList, l List) bool {
	return ContainOf(hList.PtrListHead(), l.PtrListHead())
}

func ContainOf(head, elm *ListHead) bool {

	if containOf(head.Prev(), elm) {
		return true
	}
	//containOf(head, elm)
	return containOf(elm.Prev(), head)
}

func containOf(head, elm *ListHead) bool {

	c := head.Cursor()

	for c.Next() {
		if c.Pos == elm {
			return true
		}
	}

	return false
}

// Cas ... CompareAndSwap in *ListHead
func Cas(target **ListHead, old, new *ListHead) bool {
	return atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(target)),
		unsafe.Pointer(old),
		unsafe.Pointer(new))
}

func StoreListHead(dst **ListHead, src *ListHead) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(dst)),
		unsafe.Pointer(src))
}

//go:nocheckptr
// MarkListHead sets the mark bit of the link at target only while the link
// still holds old. It also succeeds when the link already holds old with the
// mark bit, so that a retried delete can mark the same link again.
func MarkListHead(target **ListHead, old *ListHead) bool {

	//mask := uintptr(^uint(0)) ^ 1
	marked := unsafe.Pointer(uintptr(unsafe.Pointer(old)) | 1)
	if atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(target)),
		unsafe.Pointer(old),
		marked) {
		return true
	}
	return atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(target))) == marked

}

const UseRecoverState = true

var RecoverStats map[string]int = map[string]int{}
var recoverMu sync.Mutex

func AddRecoverState(name string) {
	if !UseRecoverState {
		return
	}
	recoverMu.Lock()
	if _, ok := RecoverStats[name]; !ok {
		RecoverStats[name] = 0
	}
	RecoverStats[name]++
	recoverMu.Unlock()
}
