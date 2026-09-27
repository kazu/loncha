//go:build stephook

package list_head_test

import (
	"strings"
	"sync"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// Nodes p and a lie in this order between head and tail, and n and m are
// alone. Appending n after p stops at "add.cas1" of its first try, before it
// changes p.next from a to n. p is then marked for deletion to its end, so
// p.next is a with the mark. The append resumes: its first CAS fails, and its
// retry reloads p.next, a with the mark, and uses it as the expected value of
// its next CAS, which changes p.next to n without the mark, and stops at
// "add.cas2". Appending m after p now sees no mark on p.next, links m between
// p and n, and returns nil. The append of n resumes: its second CAS, on the
// prev of a with the mark, fails, its rollback fails because p.next is m, and
// its next retry links n between p and m. Both appends return nil, but p is
// deleted and neither n nor m can be reached from head. p.next must keep its
// mark, and a node an append returns nil for must be in the list.
func TestReproJ40ListaAddRetryClearsMarkOfPrevNext(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 6)
	head, tail := &e[0], &e[5]
	p, a, n, m := &e[1], &e[2], &e[3], &e[4]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "a": a}, "p", "a")
	n.Init()
	m.Init()
	names[n], names[m] = "n", "m"

	s := newStepper(t)
	c1 := s.stopAt("add.cas1", n)
	doneN, errN := goDo(func() error { _, err := p.Append(n); return err })
	c1.waitReached(t)
	if err := p.MarkForDelete(); err != nil {
		t.Fatalf("delete p: %v", err)
	}
	c2 := s.stopAt("add.cas2", n)
	c1.Release()
	// an append that sees the mark on p.next returns without its second CAS
	select {
	case <-c2.reached:
	case <-doneN:
	}

	if got := p.DirectNext(); got == got.WithOutMark() {
		t.Errorf("the retry of the append changed p.next of the deleted p from a with the mark to %s without it", names[got])
	}
	_, errM := p.Append(m)
	if errM == nil {
		t.Errorf("Append(m) after the deleted p returned nil")
	}

	c2.Release()
	waitClosed(t, doneN, "append n")
	t.Logf("Append(n) returned %v, Append(m) returned %v", *errN, errM)

	if got := p.DirectNext(); got == got.WithOutMark() {
		t.Errorf("p.next of the deleted p is %s without the mark", names[got])
	}
	fwd, bwd, err := walk(names, head, tail)
	if err != nil || strings.Join(fwd, " ") != strings.Join(bwd, " ") {
		t.Errorf("forward %q, backward %q, err %v", fwd, bwd, err)
	}
	in := " " + strings.Join(fwd, " ") + " "
	for _, c := range []struct {
		node *list_head.ListHead
		err  error
	}{{n, *errN}, {m, errM}} {
		if c.err == nil && !strings.Contains(in, " "+names[c.node]+" ") {
			t.Errorf("the append of %s returned nil, but forward %q lacks it", names[c.node], fwd)
		}
	}
}

// The insert before a node has the same retry, but it cannot clear a mark of
// prev.next: the expected value of its first CAS, on prev.next, is always the
// node it inserts before, which has no mark. Nodes p and y lie in this order
// between head and tail, and n is alone. Inserting n before y stops at
// "add.cas1" of its first try. y is then marked for deletion to its end, so
// y.prev is p with the mark. The insert resumes: its first CAS fails, and its
// retries reload y.prev, p with the mark, and use it as the node before y.
// The only CAS that expects this value is the second, on y.prev, which runs
// only after the first CAS succeeds; the first CAS goes to the next field of
// p with the mark, which is not a link, and fails. So all tries expect y at
// prev.next, none reaches the second CAS, and y.prev keeps its mark.
func TestReproJ40ListaInsertBeforeRetryKeepsMarks(t *testing.T) {
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 5)
	head, tail := &e[0], &e[4]
	p, y, n := &e[1], &e[2], &e[3]
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "y": y}, "p", "y")
	n.Init()
	names[n] = "n"

	type try struct{ point, prev, next string }
	var (
		mu    sync.Mutex
		tries []try
	)
	s := newStepper(t)
	list_head.SetStepHook(func(point string, x, pr, nx *list_head.ListHead) {
		if x == n && (point == "add.cas1" || point == "add.cas2") {
			pn := names[pr.WithOutMark()]
			if pr != pr.WithOutMark() {
				pn += "|mark"
			}
			nn := names[nx.WithOutMark()]
			if nx != nx.WithOutMark() {
				nn += "|mark"
			}
			mu.Lock()
			tries = append(tries, try{point, pn, nn})
			mu.Unlock()
		}
		s.at(point, x, pr, nx)
	})
	c1 := s.stopAt("add.cas1", n)
	doneI, errI := goDo(func() error { _, err := y.InsertBefore(n); return err })
	c1.waitReached(t)
	if err := y.MarkForDelete(); err != nil {
		t.Fatalf("delete y: %v", err)
	}
	pNext, tailPrev := p.DirectNext(), tail.DirectPrev()
	c1.Release()
	waitClosed(t, doneI, "insert n")
	t.Logf("InsertBefore(n) returned %v after %d tries", *errI, len(tries))

	mu.Lock()
	defer mu.Unlock()
	for i, tr := range tries {
		if tr.point != "add.cas1" || tr.next != "y" {
			t.Errorf("try %d: %+v; want only add.cas1 with next y", i, tr)
		}
	}
	if got := y.DirectPrev(); got == got.WithOutMark() || got.WithOutMark() != p {
		t.Errorf("y.prev is %s, want p with the mark", names[got.WithOutMark()])
	}
	if p.DirectNext() != pNext || tail.DirectPrev() != tailPrev {
		t.Errorf("p.next %s, tail.prev %s changed from %s, %s",
			names[p.DirectNext()], names[tail.DirectPrev()], names[pNext], names[tailPrev])
	}
}
