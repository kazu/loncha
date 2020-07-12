package list_head_test

import (
	"errors"
	"fmt"
	"math/rand"
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

func PtrElement(ptr unsafe.Pointer) *Element {
	return (*Element)(ptr)
}

func TestElementOf(t *testing.T) {

	first := &Element{ID: 123, Name: "first-kun"}
	first.Init()

	second := &Element{ID: 456, Name: "second-kun"}
	second.Init()
	first.Add(&second.ListHead)

	assert.Equal(t, first.Prev(), &second.ListHead)
	assert.Equal(t, first.Next(), &second.ListHead)
	assert.Equal(t, second.Prev(), &first.ListHead)
	assert.Equal(t, second.Next(), &first.ListHead)

	psecond := PtrElement(list_head.ElementOf(second, &second.ListHead))
	assert.Equal(t, "second-kun", psecond.Name)

}

func TestAddWithConcurrent(t *testing.T) {
	list_head.MODE_CONCURRENT = true

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

func TestDelete(t *testing.T) {
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

func TestDeleteWithConcurrent(t *testing.T) {
	list_head.MODE_CONCURRENT = true
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
	var list Hoge
	list.Init()

	hoge := Hoge{ID: 1, Name: "aaa"}
	hoge.Init()
	list.Add(&hoge)

	hoge2 := Hoge{ID: 2, Name: "bbb"}
	hoge2.Init()

	hoge.Add(&hoge2)

	assert.Equal(t, hoge.Next().ID, 2)
	assert.Equal(t, hoge.Len(), 2)
	assert.Equal(t, hoge.Next().Len(), 2)
}

func TestNext(t *testing.T) {
	list_head.MODE_CONCURRENT = true

	var head list_head.ListHead

	head.Init()

	marked := 0

	for i := 0; i < 10; i++ {
		e := &list_head.ListHead{}
		e.Init()
		head.Add(e)
	}

	elm := &head

	for {
		fmt.Printf("1: elm=%s\n", elm.Pp())
		if elm == elm.Next() {
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
		if elm == elm.Next() {
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
		if elm == elm.Next() {
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

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fmt.Printf("====START TEST(%s)===\n", test.Name)
			var list list_head.ListHead
			list.Init()
			for i := 0; i < test.Count; i++ {
				e := makeElement()
				list.Add(e)

				found := loncha.Contain(&test.marked, func(idx int) bool {
					return test.marked[idx] == i
				})
				if found {
					e.MarkForDelete()
				}
			}
			//list.DeleteMarked()
			if list.Len() != test.Count-len(test.marked) {
				t.Errorf("missmatch len=%d cnt=%d marked=%d", list.Len(), test.Count, len(test.marked))
			}
			fmt.Printf("====END TEST(%s)===\n", test.Name)
		})
	}
}

func TestNext1(t *testing.T) {

	list_head.MODE_CONCURRENT = true

	var head list_head.ListHead

	head.Init()
	e := &list_head.ListHead{}
	assert.Equal(t, &head, head.Next1())
	e.Init()
	head.Add(e)
	e.MarkForDelete()

	assert.Equal(t, &head, head.Next1())
	assert.Equal(t, 0, head.Len())

	e2 := &list_head.ListHead{}
	e2.Init()
	head.Add(e2)

	assert.Equal(t, e2, head.Next1())
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
				var head list_head.ListHead
				head.Init()

				doneCh := make(chan bool, test.Concurrent)

				lists := []*list_head.ListHead{}

				for i := 0; i < test.Concurrent; i++ {
					e := makeElement()
					head.Add(e)
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
func TestConcurrentAddAndDelete(t *testing.T) {
	list_head.MODE_CONCURRENT = true
	const concurrent int = 100

	var head list_head.ListHead
	var other list_head.ListHead

	head.Init()
	other.Init()

	fmt.Printf("start head=%s other=%s\n", head.P(), other.P())

	doneCh := make(chan bool, concurrent)

	cond := func() {
		if concurrent < head.Len()+other.Len() {
			//fmt.Println("invalid")
			assert.True(t, false, head.Len()+other.Len())
		}
	}
	_ = cond

	for i := 0; i < concurrent; i++ {
		go func(i int) {

			e := &list_head.ListHead{}
			e.Init()
			fmt.Printf("idx=%5d Init e=%s len(head)=%d len(other)=%d\n",
				i, e.P(), head.Len(), other.Len())
			len := head.Len()
			head.Add(e)
			if e.Front() != &head {
				fmt.Printf("!!!!\n")
			}

			assert.True(t, list_head.ContainOf(&head, e))

			for i := 0; i < 3; i++ {
				ee := &list_head.ListHead{}
				ee.Init()
				head.Add(ee)
			}

			//cond()
			fmt.Printf("idx=%5d Add e=%s last=%5v before_len(head)=%d len(head)=%d len(other)=%d\n",
				i, e.P(), e.IsLast(), len, head.Len(), other.Len())
			before_len := head.Len()
			for {

				if e.Delete() != nil {
					break
				}

				//if e.DeleteWithCas(e.Prev()) == nil {
				//	break
				//}
				fmt.Printf("delete all marked head=%s e=%s\n", head.Pp(), e.P())
				head.DeleteMarked()
				fmt.Printf("after marked gc head=%s e=%s\n", head.Pp(), e.P())
				if !list_head.ContainOf(&head, e) {
					break
				}
				//fmt.Printf("????")
			}
			if list_head.ContainOf(&head, e) {
				fmt.Printf("!!!!\n")
			}
			if before_len < head.Len() {
				fmt.Printf("invalid increase? idx=%d \n", i)
			}
			assert.False(t, list_head.ContainOf(&head, e))
			assert.Equal(t, e, e.Next())
			assert.Equal(t, e, e.Prev())

			//cond()

			fmt.Printf("idx=%5d Delete e=%s len(head)=%d len(other)=%d\n",
				i, e.Pp(), head.Len(), other.Len())
			e.Init()
			//assert.False(t, ContainOf(&head, e))

			before_e := e.Pp()
			other.Add(e)
			assert.False(t, list_head.ContainOf(&head, e))
			assert.True(t, list_head.ContainOf(&other, e))
			//cond()

			fmt.Printf("idx=%5d Move before_e=%s e=%s len(head)=%d len(other)=%d\n",
				i, before_e, e.Pp(), head.Len(), other.Len())

			doneCh <- true
		}(i)

	}
	for i := 0; i < concurrent; i++ {
		<-doneCh
	}

	head.DeleteMarked()
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
