//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes x, l and n lie in this order between head and tail. Deleting l reads
// prev x and next n and stops at "del.begin". Inserting N before n then runs
// to the end: it swaps l.next from n to N and n.prev from l to N. Deleting l
// goes on with MarkListHead(&l.next, n), which compares l.next with the value
// it reads right there, N, instead of n, so it succeeds and stores n|1 over
// the link to N. The delete then links x to n and returns no error, and the
// forward walk loses N while the backward walk reaches l and its marked prev
// link x|1.
func TestListaMarkForDeleteKeepsInsertAfterNode(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[4]
	x, l, n, N := &e[1], &e[2], &e[3], &e[5]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"x": x, "l": l, "n": n}, "x", "l", "n")
	N.Init()
	names[N] = "N"

	s := newStepper(t)
	stop := s.stopAt("del.begin", l)
	done, errL := goDo(func() error { return l.MarkForDelete() })
	stop.waitReached(t)
	if _, err := n.InsertBefore(N); err != nil {
		t.Fatalf("insert N before n: %v", err)
	}
	if l.DirectNext() != N || n.DirectPrev() != N {
		t.Fatalf("l.next %p, n.prev %p, want both N %p", l.DirectNext(), n.DirectPrev(), N)
	}
	stop.Release()
	waitClosed(t, done, "delete l")
	if *errL != nil {
		t.Fatalf("delete l: %v", *errL)
	}

	if got := x.DirectNext(); got != N {
		t.Errorf("x.next = %q, want N", names[got])
	}
	assertLinked(t, names, head, tail, "x", "N", "n")
}

// Nodes p, a and y lie in this order between head and tail. Deleting a marks
// a.next as y|1 and stops at "del.nextMarked", with prev p read. Appending N
// after p then runs to the end: it swaps p.next from a to N and a.prev from p
// to N. (a.InsertBefore(N) would refuse with ErrMarked, as it checks a.next.)
// Deleting a goes on with MarkListHead(&a.prev, p), which compares a.prev
// with N, the value it reads right there, so it succeeds and stores p|1 over
// the link to N. The delete then links y back to p and returns no error, and
// the backward walk loses N while the forward walk reaches a and its marked
// next link y|1.
func TestListaMarkForDeleteKeepsInsertBeforeNode(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[4]
	p, a, y, N := &e[1], &e[2], &e[3], &e[5]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "a": a, "y": y}, "p", "a", "y")
	N.Init()
	names[N] = "N"

	s := newStepper(t)
	stop := s.stopAt("del.nextMarked", a)
	done, errA := goDo(func() error { return a.MarkForDelete() })
	stop.waitReached(t)
	if _, err := p.Append(N); err != nil {
		t.Fatalf("append N after p: %v", err)
	}
	if p.DirectNext() != N || a.DirectPrev() != N {
		t.Fatalf("p.next %p, a.prev %p, want both N %p", p.DirectNext(), a.DirectPrev(), N)
	}
	stop.Release()
	waitClosed(t, done, "delete a")
	if *errA != nil {
		t.Fatalf("delete a: %v", *errA)
	}

	if got := y.DirectPrev(); got != N {
		t.Errorf("y.prev = %q, want N", names[got])
	}
	assertLinked(t, names, head, tail, "p", "N", "y")
}
