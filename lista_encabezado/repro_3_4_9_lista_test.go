//go:build stephook

package list_head_test

import (
	"fmt"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// newListaInsertDeleteList links p, a and y in this order between head and
// tail, and returns them with an Inited node n that is in no list, and the
// names of all of them.
func newListaInsertDeleteList(t *testing.T) (head, tail, p, a, y, n *list_head.ListHead, names map[*list_head.ListHead]string) {
	t.Helper()
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail = &e[0], &e[5]
	p, a, y, n = &e[1], &e[2], &e[3], &e[4]
	names = newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "a": a, "y": y}, "p", "a", "y")
	n.Init()
	names[n] = "n"
	return
}

// listaAlone returns an error unless neither link of n points to a node
// in names.
func listaAlone(names map[*list_head.ListHead]string, n *list_head.ListHead) error {
	next, prev := n.DirectNext(), n.DirectPrev()
	_, nextIn := names[next]
	_, prevIn := names[prev]
	if nextIn || prevIn {
		return fmt.Errorf("%s is not alone: next %s, prev %s", names[n], names[next], names[prev])
	}
	return nil
}

// deleteAndInit deletes a the way elist Delete does: MarkForDelete, then Init
// once IsSafety allows it.
func deleteAndInit(a *list_head.ListHead) error {
	if err := a.MarkForDelete(); err != nil {
		return err
	}
	return list_head.InitAfterSafety(100)(a)
}

// Nodes p, a and y lie in this order between head and tail. Inserting n
// before a passes the mark check of a and stops at "insert.begin". a is then
// deleted to its end, so p.next is y. The insert resumes: it reads a.prev,
// which is p with the mark bit, as the node before a, and its first CAS
// fails on every one of its 100 tries. InsertBefore drops that error and
// returns nil although n is not linked. The insert must return an error,
// and n must stay out of the list.
func TestListaInsertBeforeNodeMarkedBeforeFirstCAS(t *testing.T) {
	head, tail, _, a, _, n, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	ins := s.stopAt("insert.begin", n)
	doneI, errI := goDo(func() error { _, err := a.InsertBefore(n); return err })
	ins.waitReached(t)
	if err := a.MarkForDelete(); err != nil {
		t.Fatalf("delete a: %v", err)
	}
	ins.Release()
	waitClosed(t, doneI, "insert n")

	assertLinked(t, names, head, tail, "p", "y")
	if err := listaAlone(names, n); err != nil {
		t.Error(err)
	}
	if *errI == nil {
		t.Error("InsertBefore(n) returned no error, but n is not linked")
	}
}

// Nodes p, a and y lie in this order between head and tail. Inserting n
// before a stops at "add.cas2" after its first CAS: p.next is n and a.prev
// is still p. Deleting a, in another goroutine, marks a and then waits for
// the insert; a delete that does not wait runs to its end without relinking
// p.next, which is n. The insert resumes: its second CAS fails on the mark of a.prev, and its
// rollback puts p.next back to a; its retries read p with the mark bit as
// the node before a and fail. InsertBefore returns nil, and p.next is left
// at the deleted a. The list must hold p and y linked both ways, a must be
// safe to reuse, and the insert must return an error.
func TestListaInsertBeforeNodeDeletedBetweenCASes(t *testing.T) {
	head, tail, _, a, _, n, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	ins := s.stopAt("add.cas2", n)
	doneI, errI := goDo(func() error { _, err := a.InsertBefore(n); return err })
	ins.waitReached(t)
	// the delete of a waits for the insert of n next to it
	marked := s.stopAt("del.marked", a)
	doneD, errD := goDo(func() error { return a.MarkForDelete() })
	marked.waitReached(t)
	marked.Release()
	ins.Release()
	waitClosed(t, doneI, "insert n")
	waitClosed(t, doneD, "delete a")
	if *errD != nil {
		t.Fatalf("delete a: %v", *errD)
	}

	assertLinked(t, names, head, tail, "p", "y")
	if safe, _ := a.IsSafety(); !safe {
		t.Error("a is not safe to reuse after its delete")
	}
	if *errI == nil {
		t.Error("InsertBefore(n) returned no error, but n is not linked")
	}
	if err := listaAlone(names, n); err != nil {
		t.Error(err)
	}
}

// Nodes p, a and y lie in this order between head and tail. Inserting n
// before a passes the mark check of a and stops at "insert.begin". a is then
// deleted and Inited, so a.prev is a new start node s and s.next is a. The
// insert resumes: it reads s as the node before a, and both CASes of
// listAddWitCas(n, s, a) succeed. InsertBefore returns nil, and n is linked
// between s and a, outside the list. The insert must return an error, and n
// must not link to a.
func TestListaInsertBeforeNodeInitedBeforeFirstCAS(t *testing.T) {
	head, tail, _, a, _, n, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	ins := s.stopAt("insert.begin", n)
	doneI, errI := goDo(func() error { _, err := a.InsertBefore(n); return err })
	ins.waitReached(t)
	// a delete of a waits for an insert of n between its CASes
	marked := s.stopAt("del.marked", a)
	doneD, errD := goDo(func() error { return deleteAndInit(a) })
	marked.waitReached(t)
	marked.Release()
	ins.Release()
	waitClosed(t, doneI, "insert n")
	waitClosed(t, doneD, "delete a")
	if *errD != nil {
		t.Fatalf("delete a: %v", *errD)
	}

	assertLinked(t, names, head, tail, "p", "y")
	if *errI == nil {
		t.Error("InsertBefore(n) returned no error, but n is not linked")
	}
	if err := listaAlone(names, n); err != nil {
		t.Error(err)
	}
}

// Nodes p, a and y lie in this order between head and tail. Inserting n
// before a stops at "add.cas2" after its first CAS: p.next is n and a.prev
// is still p. a is then deleted and Inited in another goroutine, which waits
// for the insert; a delete that does not wait skips relinking
// p.next, which is n, and IsSafety lets Init run. The insert resumes: its
// second CAS fails on the new a.prev, its rollback puts p.next back to a,
// and its retry links n before the Inited a. InsertBefore returns nil, and
// p.next is the Inited a. The list must hold p and y linked both ways, the
// insert must return an error, and n must not link to a.
func TestListaInsertBeforeNodeInitedBetweenCASes(t *testing.T) {
	head, tail, _, a, _, n, names := newListaInsertDeleteList(t)

	s := newStepper(t)
	ins := s.stopAt("add.cas2", n)
	doneI, errI := goDo(func() error { _, err := a.InsertBefore(n); return err })
	ins.waitReached(t)
	// a delete of a waits for an insert of n between its CASes
	marked := s.stopAt("del.marked", a)
	doneD, errD := goDo(func() error { return deleteAndInit(a) })
	marked.waitReached(t)
	marked.Release()
	ins.Release()
	waitClosed(t, doneI, "insert n")
	waitClosed(t, doneD, "delete a")
	if *errD != nil {
		t.Fatalf("delete a: %v", *errD)
	}

	assertLinked(t, names, head, tail, "p", "y")
	if *errI == nil {
		t.Error("InsertBefore(n) returned no error, but n is not linked")
	}
	if err := listaAlone(names, n); err != nil {
		t.Error(err)
	}
}
