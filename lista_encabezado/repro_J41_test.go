//go:build stephook

package list_head_test

import (
	"strings"
	"sync/atomic"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes p and a lie in this order between head and tail, and n is alone.
// Appending n after p stops at "add.cas1" of its first try, before it
// changes p.next from a to n. p is then marked for deletion to its end, so
// p.next is a with the mark, and a.prev is head. The append resumes and every
// one of its tries fails: the first on p.next, and each retry on the second
// CAS, which goes to the prev of a with the mark. add gives up after 100
// tries and prints "over retry", but returns nothing, and Append returns nil
// although n is not linked. Append must return an error unless n is in the
// list.
func TestReproJ41ListaAppendDropsOverRetry(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 5)
	head, tail := &e[0], &e[4]
	p, a, n := &e[1], &e[2], &e[3]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "a": a}, "p", "a")
	n.Init()
	names[n] = "n"

	var tries int32
	s := newStepper(t)
	list_head.SetStepHook(func(point string, x, pr, nx *list_head.ListHead) {
		if point == "add.cas1" && x == n {
			atomic.AddInt32(&tries, 1)
		}
		s.at(point, x, pr, nx)
	})
	c1 := s.stopAt("add.cas1", n)
	doneN, errN := goDo(func() error { _, err := p.Append(n); return err })
	c1.waitReached(t)
	if err := p.MarkForDelete(); err != nil {
		t.Fatalf("delete p: %v", err)
	}
	c1.Release()
	waitClosed(t, doneN, "append n")

	fwd, bwd, err := walk(names, head, tail)
	t.Logf("Append(n) returned %v after %d tries; forward %q, backward %q, err %v",
		*errN, atomic.LoadInt32(&tries), fwd, bwd, err)
	if *errN == nil && !strings.Contains(" "+strings.Join(fwd, " ")+" ", " n ") {
		t.Errorf("Append(n) returned nil after %d tries, but n is not in the list", atomic.LoadInt32(&tries))
	}
	assertLinked(t, names, head, tail, "a")
}
