//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

func newSafetyList(t *testing.T) (x, l, y *list_head.ListHead, names map[*list_head.ListHead]string) {
	t.Helper()
	useConcurrentMode(t)
	e := make([]list_head.ListHead, 7)
	head, tail := &e[0], &e[6]
	w, z := &e[1], &e[5]
	x, l, y = &e[2], &e[3], &e[4]
	names = newStepList(t, head, tail,
		map[string]*list_head.ListHead{"w": w, "x": x, "l": l, "y": y, "z": z},
		"w", "x", "l", "y", "z")
	return x, l, y, names
}

// isSafetyAt stops the delete of l at point, runs whileStopped there, and then
// calls l.IsSafety. It returns what IsSafety returned and the nodes IsSafety
// took for the one before l and the one after l, as it passed them to
// "safety.nodes". The delete then goes on to its end.
func isSafetyAt(t *testing.T, names map[*list_head.ListHead]string, l *list_head.ListHead, point string, whileStopped func()) (safe bool, prev, next *list_head.ListHead) {
	t.Helper()
	s := newStepper(t)
	list_head.SetStepHook(func(p string, a, b, c *list_head.ListHead) {
		if p == "safety.nodes" && a == l {
			prev, next = b, c
		}
		s.at(p, a, b, c)
	})
	stop := s.stopAt(point, l)
	done, errL := goDo(func() error { return l.MarkForDelete() })
	stop.waitReached(t)
	whileStopped()
	safe, _ = l.IsSafety()
	stop.Release()
	waitClosed(t, done, "delete l")
	if *errL != nil {
		t.Fatalf("delete l: %v", *errL)
	}
	if prev == nil || next == nil {
		t.Fatal("IsSafety did not reach \"safety.nodes\"")
	}
	t.Logf("at %q IsSafety of l took %s before l and %s after l", point, names[prev], names[next])
	return safe, prev, next
}

// Nodes w, x, l, y and z lie in this order between head and tail. Deleting l
// stops at "del.marked": l.prev is x|1 and l.next is y|1, x and y are not
// marked, and x.next and y.prev still link to l. IsSafety of l takes
// PrevNoM(x|1) and NextNoM(y|1) for the nodes whose links it checks. PrevNoM
// sees the mark on x|1, which is the mark of l, and goes on to x.prev, so it
// passes over the live node x and returns w; NextNoM likewise passes over y
// and returns z. IsSafety then checks w.next and z.prev, which are x and y,
// never the links of x and y that still point to l, and reports l safe. The
// node IsSafety takes on each side must be the neighbor of l, and PrevNoM and
// NextNoM must stop at the neighbor that is not marked.
func TestListaIsSafetyChecksNeighborsOfNeighbors(t *testing.T) {
	x, l, y, names := newSafetyList(t)

	var prevNoM, nextNoM *list_head.ListHead
	safe, prev, next := isSafetyAt(t, names, l, "del.marked", func() {
		if x.IsMarked() || y.IsMarked() {
			t.Fatalf("x marked %v, y marked %v, want neither", x.IsMarked(), y.IsMarked())
		}
		if x.DirectNext() != l || y.DirectPrev() != l {
			t.Fatalf("x.next %p, y.prev %p, want both l %p", x.DirectNext(), y.DirectPrev(), l)
		}
		prevNoM = list_head.PrevNoM(l.DirectPrev())
		nextNoM = list_head.NextNoM(l.DirectNext())
	})

	if prev != x || next != y {
		t.Errorf("IsSafety of l checks the links of %s and %s, want its neighbors x and y", names[prev], names[next])
	}
	if prevNoM != x {
		t.Errorf("PrevNoM(l.prev) = %s, want x: it passes over the live node x", names[prevNoM])
	}
	if nextNoM != y {
		t.Errorf("NextNoM(l.next) = %s, want y: it passes over the live node y", names[nextNoM])
	}
	if safe {
		t.Error("IsSafety() = true while x and y link to l")
	}
}

// Nodes w, x, l, y and z lie in this order between head and tail. Deleting l
// stops at "del.nextMarked": l.next is y|1 and l.prev is still x, not
// marked. IsSafety of l takes PrevNoM(x), which returns x as the link has no
// mark, and NextNoM(y|1), which passes over the live node y and returns z. So
// IsSafety passes over the neighbor only on the side whose link to it carries
// the mark of l, which is what PrevNoM and NextNoM do with a marked link, and
// not a choice of IsSafety. The node IsSafety takes after l must be y.
func TestListaIsSafetyPassesOverOnlyMarkedSide(t *testing.T) {
	x, l, y, names := newSafetyList(t)

	_, prev, next := isSafetyAt(t, names, l, "del.nextMarked", func() {
		if x.IsMarked() || y.IsMarked() {
			t.Fatalf("x marked %v, y marked %v, want neither", x.IsMarked(), y.IsMarked())
		}
	})
	if prev != x {
		t.Errorf("IsSafety of l took %s before l, want x", names[prev])
	}
	if next != y {
		t.Errorf("IsSafety of l took %s after l, want y: NextNoM passes over y", names[next])
	}
}
