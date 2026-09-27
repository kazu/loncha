//go:build stephook

package list_head_test

import (
	"testing"
)

// This is the mirror of hold 1: there p.next is left at an initialized node,
// here y.prev is. Nodes p, a and y lie in this order between head and tail.
// Deleting a reads p and y as its neighbors and stops at "del.begin", before
// it marks either link. p is then deleted and initialized once IsSafety
// allows it: head.next becomes a and a.prev becomes head, IsSafety(p) looks
// only at head and y, and p.Init gives p new end nodes. Deleting a resumes
// with the p it read: it marks a.prev as p, although a.prev is head, finds no
// link from p or the node before p to a, so head.next stays at a, and changes
// y.prev from a to p, a node that is initialized. It returns nil. y.prev must
// not point to the initialized p, and the list must hold only y, walking
// forward and backward.
func TestReproJ39ListaDeleteRelinksToInitedPrev(t *testing.T) {
	head, tail, p, a, y, _, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	da := s.stopAt("del.begin", a)
	doneA, errA := goDo(func() error { return a.MarkForDelete() })
	da.waitReached(t)

	if err := deleteAndInit(p); err != nil {
		t.Fatalf("delete p: %v", err)
	}
	if _, in := names[p.DirectNext()]; in {
		t.Fatalf("p is not initialized: p.next %s", names[p.DirectNext()])
	}

	da.Release()
	waitClosed(t, doneA, "delete a")
	if *errA != nil {
		t.Fatalf("delete a: %v", *errA)
	}

	if y.DirectPrev() == p {
		t.Errorf("y.prev is the initialized p; head.next is %s", names[head.DirectNext()])
	}
	assertLinked(t, names, head, tail, "y")
}
