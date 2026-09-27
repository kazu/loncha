//go:build stephook && race

package list_head_test

import (
	"runtime"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// runOneAfterOther runs first to its end and then second, each in its own
// goroutine, with nothing that orders second after first for the race
// detector. The test goroutine waits for first to end by watching the number
// of goroutines, which the race detector does not take as synchronization;
// waiting on a channel, a WaitGroup or an atomic would order second after
// first and hide the race. So the only happens-before edges between the two
// are the ones the list code makes itself.
func runOneAfterOther(first, second func()) {
	before := runtime.NumGoroutine()
	doneFirst, doneSecond := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(doneFirst)
		first()
	}()
	for runtime.NumGoroutine() > before {
		runtime.Gosched()
	}
	go func() {
		defer close(doneSecond)
		second()
	}()
	<-doneFirst
	<-doneSecond
}

// newPlainAccessList links p, a and y in this order between head and tail,
// and returns them with a single node n and the names of all of them.
func newPlainAccessList(t *testing.T) (head, tail, a, y, n *list_head.ListHead, names map[*list_head.ListHead]string) {
	t.Helper()
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail = &e[0], &e[5]
	p, a, y, n := &e[1], &e[2], &e[3], &e[4]
	names = newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "a": a, "y": y}, "p", "a", "y")
	n.Init()
	names[n] = "n"
	return head, tail, a, y, n, names
}

// Nodes p, a and y lie in this order between head and tail, and n is a single
// node. Deleting a runs to its end first: it moves p.next from a to y and
// y.prev from a to p with CAS, and after each CAS it reads the same link
// plainly again, in the loop over its two neighbors and in the check that
// nothing links to a any more. Inserting n before y runs next: it reads p as
// the node before y, and its CASes move p.next from y to n and y.prev from p
// to n. Each CAS of the insert takes in the delete's CAS on the same link, but
// nothing orders it after the plain reads that follow that CAS, so the race
// detector reports the plain reads of p.next and y.prev in MarkForDelete
// against the CASes of the insert. The delete must read the links
// atomically, and the list must then hold p, n and y.
func TestListaMarkForDeleteReadsLinksAtomically(t *testing.T) {
	head, tail, a, y, n, names := newPlainAccessList(t)

	var errDelete, errInsert error
	runOneAfterOther(
		func() { errDelete = a.MarkForDelete() },
		func() { _, errInsert = y.InsertBefore(n) },
	)
	if errDelete != nil {
		t.Fatalf("delete a: %v", errDelete)
	}
	if errInsert != nil {
		t.Fatalf("insert n before y: %v", errInsert)
	}

	assertLinked(t, names, head, tail, "p", "n", "y")
}

// Nodes p, a and y lie in this order between head and tail, and n is a single
// node. Inserting n before y runs to its end first: it reads a as the node
// before y, and its CASes move a.next from y to n and y.prev from a to n.
// Deleting a runs next, and before any atomic access to a it checks in
// canPurge that a links to other nodes, reading a.prev and a.next plainly.
// Nothing orders that read of a.next after the insert's CAS on it, so the race
// detector reports the plain read in canPurge against the CAS of the insert.
// It also reports the plain read of y.prev in the loop over the neighbors of
// a against the insert's CAS on y.prev, as the delete takes y for a neighbor
// and has not read y.prev atomically before. The delete must read the links
// atomically, and the list must then hold p, n and y.
func TestListaCanPurgeReadsLinksAtomically(t *testing.T) {
	head, tail, a, y, n, names := newPlainAccessList(t)

	var errDelete, errInsert error
	runOneAfterOther(
		func() { _, errInsert = y.InsertBefore(n) },
		func() { errDelete = a.MarkForDelete() },
	)
	if errInsert != nil {
		t.Fatalf("insert n before y: %v", errInsert)
	}
	if errDelete != nil {
		t.Fatalf("delete a: %v", errDelete)
	}

	assertLinked(t, names, head, tail, "p", "n", "y")
}

// Nodes p, a and y lie in this order between head and tail, and n is a single
// node. Inserting n before y runs to its end first: its CASes move a.next
// from y to n and y.prev from a to n. A reader in the SkipMark mode runs
// next: a.Next reads a.next plainly in nextSkipMark, and y.Prev reads y.prev
// plainly in prevSkipMark. Nothing orders those reads after the insert's
// CASes, so the race detector reports each plain read against the CAS on the
// same link. The reader must read the links atomically, and it must then
// find n after a and before y.
func TestListaSkipMarkReadsLinksAtomically(t *testing.T) {
	head, tail, a, y, n, names := newPlainAccessList(t)

	var errInsert error
	var afterA, beforeY *list_head.ListHead
	runOneAfterOther(
		func() { _, errInsert = y.InsertBefore(n) },
		func() {
			afterA = a.Next(list_head.Trav(list_head.TravSkipMark))
			beforeY = y.Prev(list_head.Trav(list_head.TravSkipMark))
		},
	)
	if errInsert != nil {
		t.Fatalf("insert n before y: %v", errInsert)
	}
	if afterA != n || beforeY != n {
		t.Errorf("a.Next is %s and y.Prev is %s, want n for both", names[afterA], names[beforeY])
	}

	assertLinked(t, names, head, tail, "p", "a", "n", "y")
}

// Nodes p, a and y lie in this order between head and tail, and n is a single
// node. Inserting n before a runs to its end first: it stores p and a in
// n.prev and n.next, and its CASes move p.next from a to n and a.prev from p
// to n. a.IsSafety runs next: it reads a.prev plainly, takes n for the node
// before a, and reads n.next plainly. Nothing orders those reads after the
// insert's stores, so the race detector reports the plain reads in IsSafety
// against the CAS on a.prev and the store to n.next. IsSafety must read the
// links atomically, and the list must then hold p, n, a and y.
func TestListaIsSafetyReadsLinksAtomically(t *testing.T) {
	head, tail, a, _, n, names := newPlainAccessList(t)

	var errInsert error
	runOneAfterOther(
		func() { _, errInsert = a.InsertBefore(n) },
		func() { a.IsSafety() },
	)
	if errInsert != nil {
		t.Fatalf("insert n before a: %v", errInsert)
	}

	assertLinked(t, names, head, tail, "p", "n", "a", "y")
}

// Nodes p, a and y lie in this order between head and tail. Purging a runs to
// its end first: MarkForDelete unlinks a, and InitAfterSafety makes a a single
// node again, with Init writing a.prev and a.next plainly. Inserting a again
// before y runs next: InsertBefore reads a.prev and a.next atomically to see
// whether a is marked, and then stores new links into them. Nothing orders
// those accesses after the plain writes of Init, so the race detector reports
// the atomic accesses of the insert against the plain writes in Init. The
// insert also reads plainly the empty nodes that Init linked to a, and its
// CASes on p.next and y.prev race with the plain reads in MarkForDelete as in
// TestListaMarkForDeleteReadsLinksAtomically. Init must write the links
// atomically, and the list must then hold p, a and y.
func TestListaInitWritesLinksAtomically(t *testing.T) {
	head, tail, a, y, _, names := newPlainAccessList(t)

	var purged *list_head.ListHead
	var errInsert error
	runOneAfterOther(
		func() { _, purged = a.Purge() },
		func() { _, errInsert = y.InsertBefore(a) },
	)
	if purged != a {
		t.Fatalf("purge a returned %p, want a", purged)
	}
	if errInsert != nil {
		t.Fatalf("insert a again before y: %v", errInsert)
	}

	assertLinked(t, names, head, tail, "p", "a", "y")
}
