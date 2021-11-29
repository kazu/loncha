package list_head_test

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/kazu/loncha"
	list_head "github.com/kazu/loncha/lista_encabezado"
)

func TestInit(t *testing.T) {
	list := list_head.ListHead{}
	list.Init()

	assert.Equal(t, &list, list.Prev())
	assert.Equal(t, &list, list.Next())

}

func TestAdd(t *testing.T) {
	first := list_head.ListHead{}
	first.Init()

	second := list_head.ListHead{}
	second.Init()

	first.Add(&second)

	assert.Equal(t, first.Prev(), &second)
	assert.Equal(t, first.Next(), &second)
	assert.Equal(t, second.Prev(), &first)
	assert.Equal(t, second.Next(), &first)

}

type Element struct {
	ID   int
	Name string
	list_head.ListHead
}

func (d *Element) Offset() uintptr {
	return unsafe.Offsetof(d.ListHead)
}

func (d *Element) PtrListHead() *list_head.ListHead {
	return &(d.ListHead)
}

func (d *Element) FromListHead(head *list_head.ListHead) list_head.List {

	return (*Element)(list_head.ElementOf(d, head))
}

func PtrElement(ptr unsafe.Pointer) *Element {
	return (*Element)(ptr)
}

func TestElementOf(t *testing.T) {
	var err error

	first := &Element{ID: 123, Name: "first-kun"}
	first.Init()

	second := &Element{ID: 456, Name: "second-kun"}
	second.Init()
	_, err = first.Append(&second.ListHead)

	assert.NoError(t, err)
	assert.Equal(t, first.Prev(), &second.ListHead)
	assert.Equal(t, first.Next(), &second.ListHead)
	assert.Equal(t, second.Prev(), &first.ListHead)
	assert.Equal(t, second.Next(), &first.ListHead)

	psecond := PtrElement(list_head.ElementOf(second, &second.ListHead))
	assert.Equal(t, "second-kun", psecond.Name)

}

func TestAddWithConcurrent(t *testing.T) {
	list_head.MODE_CONCURRENT = true

	var err error

	first := &list_head.ListHead{}
	first.Init()

	second := &list_head.ListHead{}
	second.Init()

	first, err = first.Append(second)
	assert.NoError(t, err)

	assert.Equal(t, first.Prev(), second)
	assert.Equal(t, first.Next(), second)
	assert.Equal(t, second.Prev(), first)
	assert.Equal(t, second.Next(), first)

}

func TestJoinWithConcurrent(t *testing.T) {
	list_head.MODE_CONCURRENT = true

	list1s := MakeList(3)
	list2s := MakeList(3)

	list := list1s[0]
	list2 := list2s[0]

	assert.Equal(t, 3, list.Len())
	list.Join(list2)

	assert.Equal(t, 6, list.Len())
	assert.NoError(t, list.Front().Validate())

	list1s = MakeList(3)
	list2s = MakeList(3)

	list = list1s[0]
	list2 = list2s[0]

	list.Back().Join(list2)

	assert.Equal(t, 6, list.Len())
	assert.NoError(t, list.Front().Validate())

}

func TestDelete(t *testing.T) {
	list_head.MODE_CONCURRENT = false
	first := list_head.ListHead{}
	first.Init()

	second := list_head.ListHead{}
	second.Init()

	first.Add(&second)

	assert.Equal(t, first.Prev(), &second)
	assert.Equal(t, first.Next(), &second)
	assert.Equal(t, second.Prev(), &first)
	assert.Equal(t, second.Next(), &first)

	second.Delete()

	assert.Equal(t, first.Prev(), &first)
	assert.Equal(t, first.Next(),
		&first, fmt.Sprintf("first=%+v next=%+v", &first, first.Next()))
	assert.True(t, first.Empty())
	assert.True(t, first.IsLast())
	assert.Equal(t, second.Prev(), &second)
	assert.Equal(t, second.Next(), &second)

}

func MakeList(cnt int) (elms []*list_head.ListHead) {

	elms = make([]*list_head.ListHead, 0, cnt)
	var err error
	for i := 0; i < cnt; i++ {
		elms = append(elms, &list_head.ListHead{})
		elms[i].Init()
		if i > 0 {
			elms[i-1], err = elms[i-1].Append(elms[i])
			if err != nil {
				fmt.Printf("!!!")
			}
		}
	}
	return elms
}

func ListIsNext(prev, next *list_head.ListHead) bool {

	if prev.Next() != next {
		return false
	}
	if next.Prev() != prev {
		return false
	}
	return true
}

func TestDeleteWithConcurrent(t *testing.T) {
	list_head.MODE_CONCURRENT = true

	elms := MakeList(3)
	first, second, third := elms[0], elms[1], elms[2]

	assert.Equal(t, first.Prev(), second)
	assert.Equal(t, first.Next(), second)
	assert.Equal(t, second.Prev(), first)
	assert.Equal(t, second.Next(), first)

	second.Purge()

	assert.True(t, first.IsFirst())
	assert.True(t, ListIsNext(first, third))

	assert.True(t, second.IsFirst() && second.IsLast())
	assert.True(t, third.IsLast())
	assert.Equal(t, second.Prev(), second)
	assert.Equal(t, second.Next(), second)

	elms = MakeList(3)
	first, second, third = elms[0], elms[1], elms[2]

	third.Purge()
	assert.True(t, first.IsFirst())
	assert.True(t, ListIsNext(first, second))
	assert.True(t, third.IsFirst() && third.IsLast())

	elms = MakeList(3)
	first, second, third = elms[0], elms[1], elms[2]

	first.Purge()
	assert.True(t, first.IsFirst() && first.IsLast())
	assert.True(t, ListIsNext(second, third))
	assert.True(t, third.IsLast())
	assert.True(t, second.IsFirst())

}

func TestPurgeFirstWithConcurrent(t *testing.T) {
	list_head.MODE_CONCURRENT = true

	var err error
	first := &list_head.ListHead{}
	first.Init()

	second := &list_head.ListHead{}
	second.Init()

	first, err = first.Append(second)
	assert.NoError(t, err)

	assert.Equal(t, first.Prev(), second)
	assert.Equal(t, first.Next(), second)
	assert.Equal(t, second.Prev(), first)
	assert.Equal(t, second.Next(), first)

	//first.Delete()
	first.Purge()

	assert.Equal(t, first.Prev(), first)
	assert.Equal(t, first.Next(),
		first, fmt.Sprintf("first=%+v next=%+v", first, first.Next()))
	assert.True(t, first.IsFirst() && first.IsLast())
	assert.True(t, first.IsLast())
	assert.Equal(t, second.Prev(), second)
	assert.Equal(t, second.Next(), second)

}

type Hoge struct {
	ID   int
	Name string
	list_head.ListHead
}

func NewHogeWithList(h *Hoge) *Hoge {
	h.Init()
	return h
}

func (d *Hoge) Init() {
	d.ListHead.Init()
}

func (d *Hoge) Next() *Hoge {
	if d.ListHead.Next() == nil {
		panic(errors.New("d.next is nil"))
	}
	return (*Hoge)(unsafe.Pointer(uintptr(unsafe.Pointer(d.ListHead.Next())) - unsafe.Offsetof(d.ListHead)))
}

func (d *Hoge) Prev() *Hoge {
	if d.ListHead.Next() == nil {
		panic(errors.New("d.prev is nil"))
	}
	return (*Hoge)(unsafe.Pointer(uintptr(unsafe.Pointer(d.ListHead.Prev())) - unsafe.Offsetof(d.ListHead)))
}

func (d *Hoge) Add(n *Hoge) {
	if n.ListHead.Next() == nil || n.ListHead.Prev() == nil {
		panic(errors.New("d is initialized"))
	}
	d.ListHead.Add(&n.ListHead)
}

func (d *Hoge) Delete() *Hoge {
	ptr := d.ListHead.Delete()
	return (*Hoge)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) - unsafe.Offsetof(d.ListHead)))
}

func (d *Hoge) ContainOf(ptr *list_head.ListHead) *Hoge {
	return (*Hoge)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) - unsafe.Offsetof(d.ListHead)))
}

func TestContainerListAdd(t *testing.T) {
	list_head.MODE_CONCURRENT = true
	// var list Hoge
	// list.Init()

	hoge := Hoge{ID: 1, Name: "aaa"}
	hoge.Init()
	// list.Add(&hoge)

	hoge2 := Hoge{ID: 2, Name: "bbb"}
	hoge2.Init()

	hoge.Add(&hoge2)

	assert.Equal(t, hoge.Next().ID, 2)
	assert.Equal(t, hoge.Len(), 2)
	assert.Equal(t, hoge.Next().Len(), 2)
}
func Benchmark_Profile_Next(b *testing.B) {
	list_head.MODE_CONCURRENT = true

	benckmarks := []struct {
		name   string
		before func()
		after  func()
		next   func(*list_head.ListHead) *list_head.ListHead
		prev   func(*list_head.ListHead) *list_head.ListHead
		cnt    int
	}{
		// 	name: "Direct",
		// 	next: func(head *list_head.ListHead) *list_head.ListHead {
		// 		return head.DirectNext()
		// 	},
		// 	prev: func(head *list_head.ListHead) *list_head.ListHead {
		// 		return head.DirectPrev()
		// 	},
		// 	cnt: 10000,
		// },
		// {
		// 	name: "Wait  M",
		// 	next: func(head *list_head.ListHead) *list_head.ListHead {
		// 		return head.Next(list_head.WaitNoM())
		// 	},
		// 	prev: func(head *list_head.ListHead) *list_head.ListHead {
		// 		return head.Prev(list_head.WaitNoM())
		// 	},
		// 	cnt: 10000,
		// },
		// {
		// 	name: "Normal ",
		// 	next: func(head *list_head.ListHead) *list_head.ListHead {
		// 		return head.Next()
		// 	},
		// 	prev: func(head *list_head.ListHead) *list_head.ListHead {
		// 		return head.Prev()
		// 	},
		// 	cnt: 10000,
		// },
		{
			name:   "TravD  ",
			before: func() { list_head.DefaultModeTraverse.Option(list_head.Direct()) },
			after:  func() { list_head.DefaultModeTraverse.Option(list_head.Direct()) },
			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Next()
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Prev()
			},
			cnt: 10000,
		},
	}

	for _, bm := range benckmarks {
		b.Run(bm.name, func(b *testing.B) {
			if bm.before != nil {
				bm.before()
			}
			b.ReportAllocs()

			var head list_head.ListHead
			head.InitAsEmpty()
			for i := 0; i < 10; i++ {
				e := &list_head.ListHead{}
				e.Init()
				head.DirectNext().InsertBefore(e)
			}

			b.ResetTimer()
			b.StartTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					bm.prev(bm.next(&head))
				}
			})
			b.StopTimer()
			if bm.after != nil {
				bm.after()
			}
		})

	}

}

func Benchmark_Next(b *testing.B) {
	list_head.MODE_CONCURRENT = true

	benckmarks := []struct {
		name   string
		before func()
		after  func()
		next   func(*list_head.ListHead) *list_head.ListHead
		prev   func(*list_head.ListHead) *list_head.ListHead
		cnt    int
	}{
		{
			name: "Direct ",
			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.DirectNext()
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.DirectPrev()
			},
			cnt: 10000,
		},
		{
			name:   "Direct2",
			before: func() { list_head.DefaultModeTraverse.Option(list_head.Direct()) },
			after:  func() { list_head.DefaultModeTraverse.Option(list_head.Direct()) },
			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Next()
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Prev()
			},
			cnt: 10000,
		},
		{
			name:   "Wait M2",
			before: func() { list_head.DefaultModeTraverse.Option(list_head.WaitNoM()) },
			after:  func() { list_head.DefaultModeTraverse.Option(list_head.Direct()) },

			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Next()
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Prev()
			},
			cnt: 10000,
		},
		{
			name: "Normal ",
			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Next()
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Prev()
			},
			cnt: 10000,
		},
		{
			name: "Wait  M",
			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Next(list_head.WaitNoM())
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Prev(list_head.WaitNoM())
			},
			cnt: 10000,
		},
		{
			name: "TravD  ",
			next: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Next(list_head.Direct())
			},
			prev: func(head *list_head.ListHead) *list_head.ListHead {
				return head.Prev(list_head.Direct())
			},
			cnt: 10000,
		},
	}

	for _, bm := range benckmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			if bm.before != nil {
				bm.before()
			}

			var head list_head.ListHead
			head.InitAsEmpty()
			for i := 0; i < 10; i++ {
				e := &list_head.ListHead{}
				e.Init()
				head.DirectNext().InsertBefore(e)
			}

			b.ResetTimer()
			b.StartTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					bm.prev(bm.next(&head))
				}
			})
			b.StopTimer()
			if bm.before != nil {
				bm.after()
			}
		})

	}

}

func TestNext(t *testing.T) {
	list_head.MODE_CONCURRENT = true

	var head list_head.ListHead

	head.InitAsEmpty()

	marked := 0

	for i := 0; i < 10; i++ {
		e := &list_head.ListHead{}
		e.Init()
		head.Append(e)
	}

	elm := &head

	for {
		fmt.Printf("1: elm=%s\n", elm.Pp())
		if elm.IsLast() {
			break
		}
		elm = elm.Next()
		marked++

	}

	assert.Equal(t, 10, marked)
	fmt.Println("-----")
	marked = 0
	elm = head.Next()
	//elm = &head
	for {
		fmt.Printf("2: elm=%s\n", elm.Pp())
		if elm.IsLast() {
			break
		}

		if rand.Intn(2) == 0 {
			elm2 := elm.Next()
			elm.MarkForDelete()
			marked++
			elm = elm2
			continue
		}
		elm = elm.Next()

	}
	fmt.Println("-----")
	cnt := 0
	elm = &head
	for {
		if elm.IsLast() {
			break
		}
		elm = elm.Next()
		cnt++
	}

	assert.Equal(t, 10-marked, cnt)
	assert.Equal(t, cnt, head.Len())

}

func TestNextNew(t *testing.T) {

	tests := []struct {
		Name   string
		Count  int
		marked []int
	}{
		{
			Name:   "first middle last marked",
			Count:  10,
			marked: []int{0, 5, 9},
		},
		{
			Name:   "continus marked",
			Count:  10,
			marked: []int{4, 5, 6},
		},
		{
			Name:   "continus marked in last",
			Count:  10,
			marked: []int{3, 4, 5, 8, 9},
		},
		{
			Name:   "continus marked in first",
			Count:  10,
			marked: []int{0, 1, 2, 4, 5, 6},
		},
		{
			Name:   "all deleted",
			Count:  3,
			marked: []int{0, 1, 2},
		},
	}

	makeElement := func() *list_head.ListHead {
		e := &list_head.ListHead{}
		e.Init()
		return e
	}

	list_head.MODE_CONCURRENT = true

	var err error
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fmt.Printf("====START TEST(%s)===\n", test.Name)
			list := &list_head.ListHead{}
			list.InitAsEmpty()
			for i := 0; i < test.Count; i++ {
				e := makeElement()
				list, err = list.Append(e)
				if err != nil {
					t.Error(err)
				}

				found := loncha.Contain(&test.marked, func(idx int) bool {
					return test.marked[idx] == i
				})
				if found {
					err := e.MarkForDelete()
					if err != nil && err == list_head.ErrEmpty {
						t.Errorf("fail to mark for delete err=%s", err)
					}
				}
				if list.IsPurged() {
					list = list.ActiveList()
				}
			}
			//list.DeleteMarked()
			lLen := list.Len()
			dlen := test.Count - len(test.marked)
			_, _ = lLen, dlen
			if list.Len() != test.Count-len(test.marked) {
				t.Errorf("missmatch len=%d cnt=%d marked=%d  %v",
					list.Len(), test.Count, len(test.marked), list.Len() == test.Count-len(test.marked))
			}
			fmt.Printf("====END TEST(%s)===\n", test.Name)
		})
	}
}

func TestNext1(t *testing.T) {

	list_head.MODE_CONCURRENT = true
	var err error

	head := &list_head.ListHead{}

	head.InitAsEmpty()
	e := &list_head.ListHead{}
	assert.Equal(t, head, list_head.ListWithError(head.Next1()).List())
	e.Init()
	head, err = head.Append(e)

	assert.NoError(t, err)
	assert.Equal(t, 1, head.Len())
	e.MarkForDelete()

	assert.Equal(t, head, list_head.ListWithError(head.Next1()).List())
	assert.Equal(t, 0, head.Len())

	if head.IsPurged() {
		head = head.ActiveList()
	}

	e2 := &list_head.ListHead{}
	e2.Init()
	head, _ = head.Append(e2)

	assert.Equal(t, e2, list_head.ListWithError(head.Next1()).List())
	assert.Equal(t, 1, head.Len())

}

func TestRaceCondtion(t *testing.T) {
	list_head.MODE_CONCURRENT = true
	const concurrent int = 10000

	makeElement := func() *list_head.ListHead {
		e := &list_head.ListHead{}
		e.Init()
		return e
	}

	tests := []struct {
		Name       string
		Concurrent int
		reader     func(i int, e *list_head.ListHead)
		writer     func(i int, e *list_head.ListHead)
	}{
		{
			Name:       "LoadPointer and Cas",
			Concurrent: concurrent,
			reader: func(i int, e *list_head.ListHead) {
				if i > 1 {

					next := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(e.Prev().PtrNext())))
					if uintptr(next)^1 > 0 {
						fmt.Printf("markd %d\n", i-1)
					}
				}
			},
			writer: func(i int, e *list_head.ListHead) {
				//n := e.DirectNext()
				if atomic.CompareAndSwapPointer(
					(*unsafe.Pointer)(unsafe.Pointer(e.PtrNext())),
					unsafe.Pointer(e.DirectNext()),
					unsafe.Pointer(uintptr(unsafe.Pointer(e.DirectNext()))|1)) {
					fmt.Printf("success %d\n", i)
				}
			},
		},
		{
			Name:       "LoadPointer and StorePointer",
			Concurrent: concurrent,
			reader: func(i int, e *list_head.ListHead) {
				if i > 1 {

					next := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(e.Prev().PtrNext())))
					if uintptr(next)^1 > 0 {
						fmt.Printf("markd %d\n", i-1)
					}
				}
			},
			writer: func(i int, e *list_head.ListHead) {
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(e.PtrNext())),
					unsafe.Pointer(uintptr(unsafe.Pointer(e.DirectNext()))|1))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name,
			func(t *testing.T) {
				head := &list_head.ListHead{}
				head.Init()

				doneCh := make(chan bool, test.Concurrent)

				lists := []*list_head.ListHead{}

				for i := 0; i < test.Concurrent; i++ {
					e := makeElement()
					head, _ = head.Append(e)
					lists = append(lists, e)
				}
				for i, e := range lists {

					go func(i int, e *list_head.ListHead) {

						test.reader(i, e)
						test.writer(i, e)

						doneCh <- true
					}(i, e)

				}
				for i := 0; i < test.Concurrent; i++ {
					<-doneCh
				}

			})
	}

}

func purgeResult(active *list_head.ListHead, purged *list_head.ListHead) (bool, *list_head.ListHead, *list_head.ListHead) {

	if purged == nil {
		return false, active, purged
	}
	return true, active, purged

}

func TestConcurrentLastAppend(t *testing.T) {

	list_head.MODE_CONCURRENT = true
	const (
		concurrent    int = 100   //100
		cntPerRoutine int = 10000 //60
	)
	list := list_head.ListHead{}
	list.InitAsEmpty()

	start := list.Prev()
	last := list.Next()

	var wg sync.WaitGroup

	append := func(last, e *list_head.ListHead) {
		var err2 error
		_ = err2
		var nlast, nlast3 *list_head.ListHead
		_ = nlast
		if e.IsMarked() {
			if ok, _ := e.IsSafety(); ok {
				e.Init()
			} else {
				_ = ok
			}

		}

		nlast2, err := last.Append(e)
		if err != nil {
			err2 = err
			nlast3 = nlast2.AvoidNotAppend(err2)
			nlast, err = nlast3.Append(e)
		} else {
			nlast = nlast2
		}
		assert.NoError(t, err)
		//assert.Equal(t, start.Back().Next(), last)
	}

	purge := func(start, e *list_head.ListHead) {
		success, active, purged := purgeResult(e.Purge())
		_ = active
		assert.True(t, success)
		if success {
			assert.True(t, purged.IsMarked() || purged.IsSingle())
		} else {
			_ = success
		}
	}
	_ = purge

	makeElems := func(cnt int) (elms []*list_head.ListHead) {
		elms = make([]*list_head.ListHead, cnt)
		for i := 0; i < cnt; i++ {
			e := &list_head.ListHead{}
			e.Init()
			elms[i] = e
		}
		return

	}

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			wg.Add(1)
			elms := makeElems(cntPerRoutine)

			for i := 0; i < cntPerRoutine; i++ {
				append(last, elms[i])
			}
			for i := 0; i < cntPerRoutine; i++ {
				purge(start, elms[i])
				assert.True(t, elms[i].IsMarked() || elms[i].IsSingle())
				append(last, elms[i])
			}

			defer wg.Done()
		}(i)
	}
	wg.Wait()
	stats := list_head.RecoverStats
	_ = stats
	len := start.Len()
	_ = len

	assert.Equal(t, start.Back().Next(), last)

}

func TestConcurrentAddAndDelete(t *testing.T) {
	list_head.MODE_CONCURRENT = true
	var err error
	const concurrent int = 100

	head := &list_head.ListHead{}
	other := &list_head.ListHead{}
	var wg sync.WaitGroup

	head.InitAsEmpty()
	other.InitAsEmpty()
	headPtr := uintptr(unsafe.Pointer(head))
	_ = headPtr

	fmt.Printf("start head=%s other=%s\n", head.P(), other.P())

	cond := func() {
		if concurrent < head.Len()+other.Len() {
			//fmt.Println("invalid")
			assert.True(t, false, head.Len()+other.Len())
		}
	} // Deprecated: Delete()

	_ = cond

	for i := 0; i < concurrent; i++ {
		go func(i int) {
			wg.Add(1)
			e := &list_head.ListHead{}
			e.Init()

			hlen := head.Len()
			olen := other.Len()
			fmt.Printf("idx=%5d Init e=%s len(head)=%d len(other)=%d\n",
				i, e.P(), hlen, olen)
			len := head.Len()

			// Append
			head, err = head.Append(e)
			if err != nil {
				head, err = head.AvoidNotAppend(err).Append(e)
			}
			if !head.IsFirst() {
				head = head.ActiveList().Front()
			}
			if head.Empty() && !head.Next().Empty() {
				//head = head.Front()
				//head = head.Next()
				_ = head
			}

			assert.NoError(t, err)

			assert.True(t, head.IsFirst())

			assert.Equalf(t, unsafe.Pointer(head), unsafe.Pointer(e.Front().Prev()),
				"idx=%5d e.Front()=%s != head=%s e=%s head.Empty()=%v e.Front().Empty()=%v\n",
				i, e.Front().P(), head.P(), e.P(), head.Empty(), e.Front().Empty())

			if !list_head.ContainOf(head.Front(), e) {
				t := list_head.ContainOf(head.Front(), e)
				hf := head.Front()
				ef := e.Front()
				_, _, _ = t, hf, ef
				//assert.NoError(t, head.Validate())
			}
			assert.Truef(t, list_head.ContainOf(head.Front(), e),
				"idx=%d false list_head.ContainOf(&head, e) len=before:%d, after:%d",
				i, len, head.Len())

			// Append +3
			for i := 0; i < 3; i++ {
				ee := &list_head.ListHead{}
				ee.Init()
				head, err = head.Append(ee)
				assert.NoError(t, err)
			}

			//cond()
			fmt.Printf("idx=%5d Add e=%s last=%5v before_len(head)=%d len(head)=%d len(other)=%d\n",
				i, e.P(), e.IsLast(), len, head.Len(), other.Len())
			before_len := head.Len()

			// Purge -1
			for {

				// if e.Delete() != nil {
				// 	break
				// }
				_, e = e.Purge()

				//if e.DeleteWithCas(e.Prev()) == nil {
				//	break
				//}
				fmt.Printf("delete all marked head=%s e=%s\n", head.Pp(), e.P())
				fmt.Printf("after marked gc head=%s e=%s\n", head.Pp(), e.P())
				if !list_head.ContainOf(head, e) {
					break
				}
				if head.IsPurged() {
					head = head.ActiveList()
				}
				//fmt.Printf("????")
			}
			if list_head.ContainOf(head, e) {
				fmt.Printf("!!!!\n")
			}
			if before_len < head.Len() {
				fmt.Printf("invalid increase? idx=%d before_len=%d after=%d \n", i, before_len, head.Len())
			}
			assert.False(t, list_head.ContainOf(head, e))
			assert.Equal(t, e, e.Next())
			assert.Equal(t, e, e.Prev())

			//cond()

			fmt.Printf("idx=%5d Delete e=%s len(head)=%d len(other)=%d\n",
				i, e.Pp(), head.Len(), other.Len())
			e.Init()
			//assert.False(t, ContainOf(&head, e))

			before_e := e.Pp()
			other, err = other.Append(e)
			assert.NoError(t, err)
			assert.False(t, list_head.ContainOf(head, e))
			assert.True(t, list_head.ContainOf(other, e))
			//cond()

			fmt.Printf("idx=%5d Move before_e=%s e=%s len(head)=%d len(other)=%d\n",
				i, before_e, e.Pp(), head.Len(), other.Len())

			wg.Done()
		}(i)

	}
	wg.Wait()

	headF := head.Front()
	_ = headF
	assert.NoError(t, head.Front().Validate())
	assert.NoError(t, other.Front().Validate())
	assert.Equal(t, concurrent, other.Len())
	assert.Equal(t, 3*concurrent, head.Len(), fmt.Sprintf("head=%s head.Next()=%s", head.Pp(), head.Next().Pp()))

}

func TestUnsafe(t *testing.T) {

	b := &struct {
		a *int
	}{
		a: nil,
	}
	b2 := &struct {
		a *int
	}{
		a: nil,
	}

	i := int(4)
	b.a = &i
	b2.a = &i
	//b = nil
	b.a = (*int)(unsafe.Pointer((uintptr(unsafe.Pointer(b.a)) ^ 1)))

	cc := uintptr(unsafe.Pointer(b.a))
	_ = cc
	fmt.Printf("cc=0x%x b.a=%d b2.a=%d\n", cc, *b.a, *b2.a)
	assert.True(t, true)

}
