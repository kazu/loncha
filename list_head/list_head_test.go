package list_head_test

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"

	"github.com/kazu/lonacha/list_head"
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
	assert.Equal(t, first.Next(), &first)
	assert.True(t, first.Empty())
	assert.True(t, first.IsLast())

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

	hoge := Hoge{ID: 1, Name: "aaa"}
	hoge.Init()

	hoge2 := Hoge{ID: 2, Name: "bbb"}
	hoge2.Init()

	hoge.Add(&hoge2)

	assert.Equal(t, hoge.Next().ID, 2)
}
