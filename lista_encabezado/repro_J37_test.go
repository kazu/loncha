//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes p, a, b and q lie in this order between head and tail, and a and b
// are deleted at the same time. Deleting a marks both links of a, finds
// p.next pointing to a, checks the mark of b, finds none, chooses b as the
// new p.next, and stops at "del.relinkNext", before its CAS. b is then
// deleted to its end: it skips a.next, which has the mark, and returns nil.
// Deleting a resumes: its CAS changes p.next from a to b, a node that is
// already deleted, and its check looks only for links to a, so it returns
// nil too. p.next must not point to the deleted b, and the list must hold p
// and q, walking forward and backward.
func TestReproJ37ListaDeleteRelinksToDeletedNext(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[5]
	p, a, b, q := &e[1], &e[2], &e[3], &e[4]
	names := newStepList(t, head, tail,
		map[string]*list_head.ListHead{"p": p, "a": a, "b": b, "q": q}, "p", "a", "b", "q")

	s := newStepper(t)
	da := s.stopAt("del.relinkNext", a)
	doneA, errA := goDo(func() error { return a.MarkForDelete() })
	da.waitReached(t)
	if b.IsMarked() {
		t.Fatalf("b is marked before b is deleted")
	}

	if err := b.MarkForDelete(); err != nil {
		t.Fatalf("delete b: %v", err)
	}
	da.Release()
	waitClosed(t, doneA, "delete a")
	if *errA != nil {
		t.Fatalf("delete a: %v", *errA)
	}

	if got := p.DirectNext(); got == b {
		t.Errorf("p.next is the deleted b, whose next is %s with the mark %v",
			names[b.DirectNext().WithOutMark()], b.DirectNext() != b.DirectNext().WithOutMark())
	}
	if got := q.DirectPrev(); got != p {
		t.Logf("q.prev is %s", names[got])
	}
	assertLinked(t, names, head, tail, "p", "q")
}
