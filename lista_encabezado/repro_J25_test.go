//go:build stephook && race

package list_head_test

import (
	"runtime"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// runWhileStopped runs fn in its own goroutine while another goroutine is
// stopped, and returns after fn ends. It waits for fn by watching the number
// of goroutines, which the race detector does not take as synchronization, so
// nothing orders what the stopped goroutine does after it is released after
// what fn did. The returned channel is closed when fn ends; receiving from it
// orders the receiver after fn.
func runWhileStopped(fn func()) <-chan struct{} {
	done := make(chan struct{})
	before := runtime.NumGoroutine()
	go func() {
		defer close(done)
		fn()
	}()
	for runtime.NumGoroutine() > before {
		runtime.Gosched()
	}
	return done
}

// deleteAcrossInsert deletes l while the delete is stopped at
// "del.purgeable", runs insert to its end, and then lets the delete go on.
// The stop orders the plain reads of canPurge before everything insert does,
// as the insert starts after the delete has stopped there. Nothing orders the
// rest of the delete after the insert: the test takes in the insert only
// after the delete has ended.
func deleteAcrossInsert(t *testing.T, l *list_head.ListHead, insert func() error) (errDelete, errInsert error) {
	t.Helper()
	s := newStepper(t)
	stop := s.stopAt("del.purgeable", l)
	done, errL := goDo(func() error { return l.MarkForDelete() })
	stop.waitReached(t)
	insertDone := runWhileStopped(func() { errInsert = insert() })
	stop.Release()
	waitClosed(t, done, "delete")
	waitClosed(t, insertDone, "insert")
	return *errL, errInsert
}

// Nodes p, l and y lie in this order between head and tail, and n is a single
// node. Deleting l checks in canPurge that l links to other nodes and stops
// at "del.purgeable", before it reads its neighbors. Inserting n before l runs
// to its end: its CASes move p.next from l to n and l.prev from p to n.
// Deleting l goes on and reads l.prev plainly for prev1, the first line of the
// retry loop. Nothing orders that read after the insert's CAS on l.prev, and
// the delete has not read l.prev atomically since, so the race detector
// reports the read of prev1 against the CAS. The delete must read l.prev
// atomically, and the list must then hold p, n and y.
func TestListaMarkForDeleteReadsPrev1Atomically(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[5]
	p, l, y, n := &e[1], &e[2], &e[3], &e[4]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "l": l, "y": y}, "p", "l", "y")
	n.Init()
	names[n] = "n"

	errDelete, errInsert := deleteAcrossInsert(t, l, func() error {
		_, err := l.InsertBefore(n)
		return err
	})
	if errInsert != nil {
		t.Fatalf("insert n before l: %v", errInsert)
	}
	if errDelete != nil {
		t.Fatalf("delete l: %v", errDelete)
	}

	assertLinked(t, names, head, tail, "p", "n", "y")
}

// Nodes p, l and y lie in this order between head and tail, and n is a single
// node. Deleting l checks in canPurge that l links to other nodes and stops
// at "del.purgeable", before it reads its neighbors. Inserting n before y runs
// to its end: its CASes move l.next from y to n and y.prev from l to n.
// Deleting l goes on and reads l.next plainly for next1, the second line of
// the retry loop. Nothing orders that read after the insert's CAS on l.next,
// and the delete has not read l.next atomically since, so the race detector
// reports the read of next1 against the CAS. The delete must read l.next
// atomically, and the list must then hold p, n and y.
func TestListaMarkForDeleteReadsNext1Atomically(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[5]
	p, l, y, n := &e[1], &e[2], &e[3], &e[4]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "l": l, "y": y}, "p", "l", "y")
	n.Init()
	names[n] = "n"

	errDelete, errInsert := deleteAcrossInsert(t, l, func() error {
		_, err := y.InsertBefore(n)
		return err
	})
	if errInsert != nil {
		t.Fatalf("insert n before y: %v", errInsert)
	}
	if errDelete != nil {
		t.Fatalf("delete l: %v", errDelete)
	}

	assertLinked(t, names, head, tail, "p", "n", "y")
}
