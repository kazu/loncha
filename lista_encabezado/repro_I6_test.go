//go:build stephook

package list_head_test

import (
	"testing"
)

// Nodes p, a and y lie in this order between head and tail, and n is alone.
// Inserting n before a reads p as the node before a and stops at
// "add.cas1". a is then marked for deletion and stops at "del.marked", and
// both go on together: the insert makes its first CAS, fails its second on
// the mark of a.prev and takes n out, while the delete waits for it. Before
// the fix, MarkForDelete gave up after 100 tries with a marked but still
// p.next. MarkForDelete must not return while a is linked, and the list must
// hold p and y, walking forward and backward.
func TestReproI6ListaMarkForDeleteGivesUpLinked(t *testing.T) {
	head, tail, _, a, _, n, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	ins := s.stopAt("add.cas1", n)
	doneI, _ := goDo(func() error { _, err := a.InsertBefore(n); return err })
	ins.waitReached(t)
	mk := s.stopAt("del.marked", a)
	doneD, errD := goDo(func() error { return a.MarkForDelete() })
	mk.waitReached(t)
	mk.Release()
	ins.Release()
	waitClosed(t, doneD, "delete a")
	waitClosed(t, doneI, "insert n")
	if *errD != nil {
		t.Errorf("MarkForDelete(a) returned %v", *errD)
	}
	assertLinked(t, names, head, tail, "p", "y")
}
