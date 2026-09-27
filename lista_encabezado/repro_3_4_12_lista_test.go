//go:build stephook

package list_head_test

import (
	"strings"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes p, a and y lie in this order between head and tail, and n and m are
// Inited nodes in no list. Inserting n before a stops at "add.cas2" after its
// first CAS: p.next is n, n links to p and a, and a.prev is still p. a is
// then deleted in another goroutine: it marks a and waits for the insert of
// n. The insert resumes: its second CAS fails on the mark of a.prev, and it
// stops at "add.rollback". Inserting m before n now must not link m, because
// n is half inserted. When the rollback resumes, it removes n as a delete
// does, the delete of a finishes, and the list must hold p and y. With the
// rollback that CASes p.next back to a, m is linked between p and n, that CAS
// fails, and the forward walk leaves the list at the end node of n. Every
// node that links forward to n must be the node n links back to.
func TestListaInsertBeforeRollbackAfterInsertBeforeNew(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 7)
	head, tail := &e[0], &e[6]
	p, a, y, n, m := &e[1], &e[2], &e[3], &e[4], &e[5]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "a": a, "y": y}, "p", "a", "y")
	n.Init()
	m.Init()
	names[n], names[m] = "n", "m"

	s := newStepper(t)
	cas2 := s.stopAt("add.cas2", n)
	doneI, errI := goDo(func() error { _, err := a.InsertBefore(n); return err })
	cas2.waitReached(t)
	// the delete of a waits for the insert of n next to it, so it runs in
	// its own goroutine
	marked := s.stopAt("del.marked", a)
	doneD, errD := goDo(func() error { return a.MarkForDelete() })
	marked.waitReached(t)
	marked.Release()
	rb := s.stopAt("add.rollback", n)
	cas2.Release()
	rb.waitReached(t)
	// the insert of m waits while n is half inserted
	doneM, errM := goDo(func() error { _, err := n.InsertBefore(m); return err })
	rb.Release()
	waitClosed(t, doneI, "insert n")
	waitClosed(t, doneD, "delete a")
	// after the rollback n is alone in the list of its Init, so m either
	// fails or joins n there; it must not join the list of p and y
	waitClosed(t, doneM, "insert m")
	t.Logf("inserting m before n returned %v", *errM)
	if *errD != nil {
		t.Fatalf("delete a: %v", *errD)
	}
	assertLinked(t, names, head, tail, "p", "y")

	for x, name := range names {
		if x != n && x.DirectNext() == n && n.DirectPrev() != x {
			t.Errorf("%s.next is n, but n.prev is %p and n.next is %p, which are not in the list", name, n.DirectPrev(), n.DirectNext())
		}
	}
	if fwd, bwd, err := walk(names, head, tail); err != nil {
		var back []string
		for cur := tail.DirectPrev(); cur != head && len(back) <= len(names); cur = cur.DirectPrev() {
			back = append([]string{names[cur]}, back...)
		}
		t.Errorf("%v; backward %q", err, back)
	} else if strings.Join(fwd, " ") != strings.Join(bwd, " ") {
		t.Errorf("forward %q, backward %q", fwd, bwd)
	}
	if *errI == nil {
		if _, in := names[n.DirectPrev()]; !in {
			t.Error("InsertBefore(n) returned no error, but n does not link back into the list")
		}
	}
}
