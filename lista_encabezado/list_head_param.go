// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
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
		if ok, _ := new.IsSafety(); ok {
			new.Init()
		} else {
			return head, ErrNoSafetyOnAdd
		}
	}

	nlast, err := head.append(new)
	if err == nil {
		return nlast, err
	}
	nlast2 := nlast.AvoidNotAppend(err)
	return nlast2.append(new)
}

func (head *ListHead) InsertBefore(new *ListHead) (*ListHead, error) {

	if new.IsMarked() {
		if ok, _ := new.IsSafety(); ok {
			new.Init()
		} else {
			return head, ErrNoSafetyOnAdd
		}
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

	// if head.prev.isMarkedForDeleteWithoutError() {
	// 	return head, ErrMarked
	// }
	// if !head.prev.canAdd() {
	// 	return head, ErrNotAppend
	// }
	nNode := toNode(new)
	head.insertBefore(nNode)
	return head, nil

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
	head.add(nNode)

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

	if !head.Prev().Empty() {
		return false
	}
	if !head.Next().Empty() {
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

func (head *ListHead) add(new *ListHead) {
	if MODE_CONCURRENT {
		//retry := 0
		var err error
		if !new.IsSingle() {
			fmt.Printf("Warn: insert element must be single node\n")
		}
		prev := head
		next := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.next))))
		err = retry(100, func(retry int) (finish bool, err error) {
			err = listAddWitCas(new,
				prev,
				next)
			if err == nil {
				return true, err
			}

			next = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&prev.next))))
			AddRecoverState("cas retry")
			return false, err
		})

		if err != nil {
			fmt.Printf("add(): over retry retry=%d err=%s\n", 100, err.Error())
		}

		return
	}
	listAdd(new, head, head.next)
}

func (head *ListHead) insertBefore(new *ListHead) {
	if MODE_CONCURRENT {
		//retry := 0
		var err error
		if !new.IsSingle() {
			fmt.Printf("Warn: insert element must be single node\n")
		}
		next := head
		prev := (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))
		err = retry(100, func(retry int) (finish bool, err error) {
			err = listAddWitCas(new,
				prev,
				next)
			if err == nil {
				return true, err
			}

			prev = (*ListHead)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&head.prev))))
			AddRecoverState("cas retry")
			return false, err
		})

		if err != nil {
			fmt.Printf("insertBefore(): over retry retry=%d err=%s\n", 100, err.Error())
		}

		return
	}
	listAdd(new, head.prev, head)
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

func MarkListHead(target **ListHead, old *ListHead) bool {

	//mask := uintptr(^uint(0)) ^ 1
	return atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(target)),
		unsafe.Pointer(uintptr(unsafe.Pointer(*target))),
		unsafe.Pointer(uintptr(unsafe.Pointer(old))|1))

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
