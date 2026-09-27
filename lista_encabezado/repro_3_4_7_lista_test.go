//go:build stephook

package list_head_test

import (
	"testing"
	"unsafe"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes x, l, n and s lie in this order between head and tail. Deleting l
// stops at "del.marked": l.next is n|1 and l.prev is x|1, while x.next and
// n.prev still link to l. The WaitNoMark traversal from l loads l.next, n|1,
// and asks whether n|1 is marked; IsMarked then reads its two links at n+1
// and n+9, whose bit 0 is bit 8 of n.prev (l) and of n.next (s). The nodes
// are placed so that bit 8 of l and s is 0, so n|1 looks unmarked and is
// returned as a node. The traversal back from l returns x|1 the same way,
// as bit 8 of x.prev (head) and x.next (l) is 0. Each traversal must return
// nil or one of the nodes of the list, never a marked link.
func TestListaWaitNoMarkFromMarkedNode(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 64)
	var at []*list_head.ListHead
	for i := range e {
		if uintptr(unsafe.Pointer(&e[i]))&0x100 == 0 {
			at = append(at, &e[i])
		}
	}
	if len(at) < 6 {
		t.Fatalf("only %d nodes have bit 8 of their address clear", len(at))
	}
	head, x, l, n, s, tail := at[0], at[1], at[2], at[3], at[4], at[5]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"x": x, "l": l, "n": n, "s": s}, "x", "l", "n", "s")

	st := newStepper(t)
	stop := st.stopAt("del.marked", l)
	done, _ := goDo(func() error { return l.MarkForDelete() })
	stop.waitReached(t)
	if x.DirectNext() != l || n.DirectPrev() != l {
		t.Fatalf("x.next %p, n.prev %p, want both l %p", x.DirectNext(), n.DirectPrev(), l)
	}
	next := l.Next(list_head.WaitNoM())
	prev := l.Prev(list_head.WaitNoM())
	stop.Release()
	waitClosed(t, done, "delete l")

	for _, got := range []struct {
		what string
		node *list_head.ListHead
	}{{"l.Next()", next}, {"l.Prev()", prev}} {
		if got.node == nil {
			continue
		}
		if uintptr(unsafe.Pointer(got.node))&1 != 0 {
			t.Errorf("%s waiting for no mark = %p, a marked link (n %p, x %p)", got.what, got.node, n, x)
			continue
		}
		if _, ok := names[got.node]; !ok {
			t.Errorf("%s waiting for no mark = %p, not a node of the list", got.what, got.node)
		}
	}
}
