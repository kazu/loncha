//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes x, l and y lie in this order between head and tail. Deleting l stops
// at "del.marked": l.prev is x|1 and l.next is y|1, while x.next and y.prev
// still link to l. IsSafety of l then takes PrevNoM(x|1), which passes over
// the live node x to head, and NextNoM(y|1), which passes over y to tail. It
// checks only head.next and tail.prev, finds neither marked nor linking to
// l, and reports l safe to reuse while x and y link to it. After the delete
// ends, l is safe.
func TestListaIsSafetyWhileNeighborsLinkTo(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 5)
	head, tail := &e[0], &e[4]
	x, l, y := &e[1], &e[2], &e[3]
	newStepList(t, head, tail, map[string]*list_head.ListHead{"x": x, "l": l, "y": y}, "x", "l", "y")

	s := newStepper(t)
	stop := s.stopAt("del.marked", l)
	done, errL := goDo(func() error { return l.MarkForDelete() })
	stop.waitReached(t)
	if x.DirectNext() != l || y.DirectPrev() != l {
		t.Fatalf("x.next %p, y.prev %p, want both l %p", x.DirectNext(), y.DirectPrev(), l)
	}
	during, _ := l.IsSafety()
	stop.Release()
	waitClosed(t, done, "delete l")
	if *errL != nil {
		t.Fatalf("delete l: %v", *errL)
	}
	after, _ := l.IsSafety()

	if during {
		t.Error("IsSafety() = true while x and y link to l")
	}
	if !after {
		t.Error("IsSafety() = false after l is deleted")
	}
}
