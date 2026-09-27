//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes x, a and y lie in this order between head and tail. Deleting a stops
// at "del.marked", so both links of a are marked while x.next and y.prev
// still link to a. The SkipMark traversal from x reads x.next, a, sees that
// a is marked, and calls NextNoM(x.next); x.next has no mark bit, so NextNoM
// returns a itself. The traversal from y back returns a the same way. Both
// must pass over the marked node a.
func TestListaSkipMarkPassesMarkedNode(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 5)
	head, tail := &e[0], &e[4]
	x, a, y := &e[1], &e[2], &e[3]
	newStepList(t, head, tail, map[string]*list_head.ListHead{"x": x, "a": a, "y": y}, "x", "a", "y")

	s := newStepper(t)
	stop := s.stopAt("del.marked", a)
	done, _ := goDo(func() error { return a.MarkForDelete() })
	stop.waitReached(t)
	if !a.IsMarked() {
		t.Fatal("a is not marked at del.marked")
	}
	next := x.Next(list_head.Trav(list_head.TravSkipMark))
	prev := y.Prev(list_head.Trav(list_head.TravSkipMark))
	stop.Release()
	waitClosed(t, done, "delete a")

	if next != y {
		t.Errorf("x.Next() skipping marks = %p, want y %p (a is %p)", next, y, a)
	}
	if prev != x {
		t.Errorf("y.Prev() skipping marks = %p, want x %p (a is %p)", prev, x, a)
	}
}
