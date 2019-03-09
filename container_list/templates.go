package container_list

import "github.com/clipperhouse/typewriter"

var templates = typewriter.TemplateSlice{
	list,
}

var list = &typewriter.Template{
	Name: "ContainerList",
	Text: `

func (d *{{.Name}}) Init() {
	d.ListHead.Init()
}

// Next returns the next list element or nil.
func (d *{{.Name}}) Next() *{{.Name}} {
	if d.ListHead.Next() == nil {
		panic(errors.New("d.next is nil"))
	}
	return (*{{.Name}})(unsafe.Pointer(uintptr(unsafe.Pointer(d.ListHead.Next())) - unsafe.Offsetof(d.ListHead)))
}
// Prev returns the previous list element or nil.
func (d *{{.Name}}) Prev() *{{.Name}} {
	if d.ListHead.Next() == nil {
		panic(errors.New("d.prev is nil"))
	}
	return (*{{.Name}})(unsafe.Pointer(uintptr(unsafe.Pointer(d.ListHead.Prev())) - unsafe.Offsetof(d.ListHead)))
}

// New returns an initialized list.
func New{{.Name}}List(h *{{.Name}}) *{{.Name}} {
	h.Init()
	return h
}

func (d *{{.Name}}) Len() int {
	return d.ListHead.Len()
}

func (d *{{.Name}}) Add(n *{{.Name}})  *{{.Name}} {
	d.ListHead.Add(n.ListHead)
	return n
}


func (d *{{.Name}}) Delete() *{{.Name}} {
	ptr := d.ListHead.Delete()
	return (*{{.Name}})(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) - unsafe.Offsetof(d.ListHead)))
}

func (d *{{.Name}}) Remove() *{{.Name}} {
	return d.Delete()
}

func (d *{{.Name}}) ContainOf(ptr *list_head.ListHead) *{{.Name}} {
	return (*{{.Name}})(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) - unsafe.Offsetof(d.ListHead)))
}

func (d *{{.Name}}) Front() *{{.Name}} {
	return d.ContainOf(d.ListHead.Front())
}


func (d *{{.Name}}) Back() *{{.Name}} {
	return d.ContainOf(d.ListHead.Back())
}

// PushFront inserts a new value v at the front of list l and returns e.
func (d *{{.Name}}) PushFront(v *{{.Name}}) *{{.Name}} {
	front := d.Front()
	v.Add(front)
	return v
}


// PushBack inserts a new element e with value v at the back of list l and returns e.
func (l *{{.Name}}) PushBack(v *{{.Name}}) *{{.Name}} {
	last := l.Back()
	last.Add(v)
	return v
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element of l, the list is not modified.
func (l *{{.Name}}) InsertBefore(v {{.Pointer}}{{.Name}}) *{{.Name}} {
	l.Prev().Add(v)
	return v
}

// InsertAfter inserts a new element e with value v immediately after mark and returns e.
// If mark is not an element of l, the list is not modified.
func (l *{{.Name}}) InsertAfter(v {{.Pointer}}{{.Name}}) *{{.Name}} {
	l.Next().Add(v)
	return v
}


// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
func (l *{{.Name}}) MoveToFront(v {{.Pointer}}{{.Name}}) *{{.Name}} {
	v.Remove()
	return l.PushFront(v)
}

func (l *{{.Name}}) MoveToBack(v {{.Pointer}}{{.Name}}) *{{.Name}} {
	v.Remove()
	return l.PushBack(v)
}


// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
func (l *{{.Name}}) MoveBefore(v {{.Pointer}}{{.Name}}) *{{.Name}} {
	v.Remove()
	l.Prev().Add(v)
	return v
}

// MoveAfter moves element e to its new position after mark.
// If e is not an element of l, or e == mark, the list is not modified.
func (l *{{.Name}}) MoveAfter(v {{.Pointer}}{{.Name}}) *{{.Name}} {
	v.Remove()
	l.Add(v)
	return v
}

func (l *{{.Name}}) PushBackList(other *{{.Name}}) {
	l.Back().Add(other)
	return 
}

func (l *{{.Name}}) PushBackList(other *{{.Name}}) {
	other.PushBackList(l)
	return
}

`}
