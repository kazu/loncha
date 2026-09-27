//go:build stephook

package list_head_test

import (
	"strings"
	"testing"
)

// This is the order of I5 of elist, run on lista. Nodes p, a and y lie in
// this order between head and tail, and n is alone. Inserting n before y
// reads a as the node before y, makes its first CAS, a.next from y to n, and
// stops at "add.cas2", before its second CAS, y.prev from a to n. So a.next
// is n, n.prev is a and n.next is y, but y.prev is still a. a is then deleted
// and initialized once IsSafety allows it, as elist Delete does. y is then
// marked for deletion. The insert resumes: its second CAS fails, and its
// rollback, a.next from n back to y, fails on the initialized a, so p.next is
// left at n while n links to itself again; its retries read y.prev, which has
// the mark, as the node before y, and fail. a must not be initialized while
// y.prev points to it, and in the end the list must hold p and not a or y,
// walking forward and backward, with n either linked or alone.
func TestReproJ38ListaDeleteInitsNodeStillPrevOfNext(t *testing.T) {
	head, tail, p, a, y, n, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	ins := s.stopAt("add.cas2", n)
	doneI, errI := goDo(func() error { _, err := y.InsertBefore(n); return err })
	ins.waitReached(t)
	if got := a.DirectNext(); got != n {
		t.Fatalf("after the first CAS a.next is %s, want n", names[got])
	}

	// the deletes of a and y wait for the insert of n between them, so they
	// run in their own goroutines
	mkA := s.stopAt("del.marked", a)
	doneA, errA := goDo(func() error { return deleteAndInit(a) })
	mkA.waitReached(t)
	mkY := s.stopAt("del.marked", y)
	doneY, errY := goDo(func() error { return y.MarkForDelete() })
	mkY.waitReached(t)
	mkA.Release()
	mkY.Release()

	ins.Release()
	waitClosed(t, doneI, "insert n")
	waitClosed(t, doneA, "delete a")
	waitClosed(t, doneY, "delete y")
	if *errA != nil {
		t.Errorf("delete a: %v", *errA)
	}
	if *errY != nil {
		t.Errorf("delete y: %v", *errY)
	}
	t.Logf("after deleting a and y: p.next %s, tail.prev %s", names[p.DirectNext()], names[tail.DirectPrev()])
	if got := tail.DirectPrev(); got == a {
		t.Errorf("after deleting y, tail.prev is the initialized a")
	}
	t.Logf("insert n returned %v", *errI)

	fwd, bwd, err := walk(names, head, tail)
	joined := " " + strings.Join(fwd, " ") + " "
	switch {
	case err != nil:
		t.Errorf("%v; forward %q, backward %q", err, fwd, bwd)
	case strings.Join(fwd, " ") != strings.Join(bwd, " "):
		t.Errorf("forward %q, backward %q differ", fwd, bwd)
	case !strings.Contains(joined, " p "):
		t.Errorf("forward %q lacks p", fwd)
	case strings.Contains(joined, " a ") || strings.Contains(joined, " y "):
		t.Errorf("forward %q holds a deleted node", fwd)
	case !strings.Contains(joined, " n "):
		if err := listaAlone(names, n); err != nil {
			t.Error(err)
		}
	}
}
