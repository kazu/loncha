//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes x, a, b and y lie in this order between head and tail. a and b are
// deleted at the same time: deleting a stops at "del.marked", with a.prev
// x|1 and a.next b|1. b is then deleted to its end: b.prev is a|1, so
// PrevNoM(a|1) passes over a and also over the live node x, and b relinks
// y.prev to head. Deleting a resumes: NextNoM(b|1) passes over b and also
// over the live node y, and a relinks x.next to tail. Both deletes return
// nil, and the list lost y going forward and x going backward. x and y must
// stay linked to each other in both directions.
func TestListaMarkForDeleteAdjacentKeepsNeighbors(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[5]
	x, a, b, y := &e[1], &e[2], &e[3], &e[4]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"x": x, "a": a, "b": b, "y": y}, "x", "a", "b", "y")

	s := newStepper(t)
	stop := s.stopAt("del.marked", a)
	done, errA := goDo(func() error { return a.MarkForDelete() })
	stop.waitReached(t)
	if err := b.MarkForDelete(); err != nil {
		t.Fatalf("delete b: %v", err)
	}
	stop.Release()
	waitClosed(t, done, "delete a")
	if *errA != nil {
		t.Fatalf("delete a: %v", *errA)
	}

	assertLinked(t, names, head, tail, "x", "y")
}
