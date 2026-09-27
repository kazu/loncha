//go:build stephook

package list_head_test

import (
	"runtime/debug"
	"testing"
	"unsafe"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// The tests in this file take TestConcurrentAddAndDelete apart. Each one
// replays one order that makes that test fail, and names the failure it
// explains:
//   - a nil pointer dereference in frontCc (TestReproI9FrontAfterWaitNoMarkGivesUp),
//   - a nil pointer dereference in lenCc (TestReproI9NextTakesWaitNoMarkOfAnotherCall,
//     TestReproI9WaitNoMarkStaysAfterTwoPrevCalls),
//   - "unexpected fault address" in prevWaitNoMark (TestReproI9FrontFaultsOnMarkedLink),
//   - the check at list_head_test.go:957 (TestReproI9ContainOfMissesNodeAfterCursorNodePurged),
//   - the check at list_head_test.go:946 (TestReproI9Line946CheckFailsOnCorrectList).

// callRecoverI9 calls fn on the calling goroutine with faults turned into
// panics, and returns the value of a panic out of fn (nil if none). The stack
// of the panic is logged.
func callRecoverI9(t *testing.T, fn func()) (panicked interface{}) {
	t.Helper()
	old := debug.SetPanicOnFault(true)
	defer debug.SetPanicOnFault(old)
	defer func() {
		if panicked = recover(); panicked != nil {
			t.Logf("stack of the panic:\n%s", debug.Stack())
		}
	}()
	fn()
	return nil
}

// keepTraverseDirect puts DefaultModeTraverse back to TravDirect when the test
// ends, as the default mode is shared by every test in the package.
func keepTraverseDirect(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { list_head.DefaultModeTraverse.SetType(list_head.TravDirect) })
}

// Two goroutines, list head p x tail.
//
//  1. D calls p.MarkForDelete() and stops at "del.marked": p.prev and p.next
//     carry the mark, and x.prev is still p.
//  2. T calls x.Front(). frontCc checks its loop condition
//     !head.Prev(WaitNoM()).Empty() with head = x. prevWaitNoMark(x) reads
//     x.prev = p, finds p marked 100 times in a row (D is stopped) and returns
//     nil. frontCc calls nil.Empty() and T panics with a nil pointer
//     dereference.
//
// This is the stack of the SIGSEGV in TestConcurrentAddAndDelete:
// func2 -> Front -> frontCc (loop condition) -> Empty -> prevLoad(nil).
// When D is released, x is the first node, which Front should return.
func TestReproI9FrontAfterWaitNoMarkGivesUp(t *testing.T) {
	useConcurrentMode(t)
	s := newStepper(t)
	head, tail := &list_head.ListHead{}, &list_head.ListHead{}
	p, x := &list_head.ListHead{}, &list_head.ListHead{}
	names := newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "x": x}, "p", "x")

	marked := s.stopAt("del.marked", p)
	delDone, delErr := goDo(func() error { return p.MarkForDelete() })
	marked.waitReached(t)

	var got *list_head.ListHead
	panicked := callRecoverI9(t, func() { got = x.Front() })

	marked.Release()
	waitClosed(t, delDone, "p.MarkForDelete")
	if *delErr != nil {
		t.Fatalf("p.MarkForDelete: %v", *delErr)
	}
	assertLinked(t, names, head, tail, "x")
	if panicked != nil {
		t.Fatalf("x.Front() while p.MarkForDelete stood at del.marked panicked: %v", panicked)
	}
	if got != x {
		t.Errorf("x.Front() = %p, want x %p", got, x)
	}
}

// Three goroutines, list head y p x tail. DefaultModeTraverse is one value
// shared by all goroutines, and Prev(WaitNoM()) switches it for the length of
// the call.
//
//  1. A calls tail.Prev(WaitNoM()). ListPrev sets DefaultModeTraverse to
//     TravWaitNoMark and A stops at "prev.waitNoMark", before it reads
//     tail.prev.
//  2. D calls p.MarkForDelete() and stops at "del.marked": p is marked, and
//     y.next is still p.
//  3. T calls y.Next() with no option, which should read y.next directly and
//     return p. ListNext reads the shared mode, takes nextWaitNoMark, finds p
//     marked 100 times and returns nil.
//
// This is the stack of the SIGSEGV through lenCc in TestConcurrentAddAndDelete:
// lenCc calls cur.Next() without options, gets nil and calls nil.Empty().
func TestReproI9NextTakesWaitNoMarkOfAnotherCall(t *testing.T) {
	useConcurrentMode(t)
	keepTraverseDirect(t)
	s := newStepper(t)
	head, tail := &list_head.ListHead{}, &list_head.ListHead{}
	y, p, x := &list_head.ListHead{}, &list_head.ListHead{}, &list_head.ListHead{}
	newStepList(t, head, tail, map[string]*list_head.ListHead{"y": y, "p": p, "x": x}, "y", "p", "x")

	inPrev := s.stopAt("prev.waitNoMark", tail)
	prevDone, _ := goDo(func() error { tail.Prev(list_head.WaitNoM()); return nil })
	inPrev.waitReached(t)

	marked := s.stopAt("del.marked", p)
	delDone, _ := goDo(func() error { return p.MarkForDelete() })
	marked.waitReached(t)

	got := y.Next()

	inPrev.Release()
	waitClosed(t, prevDone, "tail.Prev(WaitNoM())")
	marked.Release()
	waitClosed(t, delDone, "p.MarkForDelete")
	if got != p {
		t.Fatalf("y.Next() with no option = %p while another goroutine was in Prev(WaitNoM()), want p %p", got, p)
	}
}

// Three goroutines, list head a b y p tail. Each Prev(WaitNoM()) puts back the
// mode it saw when it started, so two calls that overlap put back the modes in
// the wrong order (a lost update, J15).
//
//  1. A calls a.Prev(WaitNoM()): it saves TravDirect, sets TravWaitNoMark and
//     stops at "prev.waitNoMark".
//  2. B calls b.Prev(WaitNoM()): it saves TravWaitNoMark (set by A) and stops
//     at "prev.waitNoMark".
//  3. A returns and puts back TravDirect. B returns and puts back
//     TravWaitNoMark. No call is running, and the mode stays TravWaitNoMark.
//  4. D calls p.MarkForDelete() and stops at "del.marked". T calls y.Next()
//     with no option and gets nil instead of p, as in
//     TestReproI9NextTakesWaitNoMarkOfAnotherCall.
func TestReproI9WaitNoMarkStaysAfterTwoPrevCalls(t *testing.T) {
	useConcurrentMode(t)
	keepTraverseDirect(t)
	s := newStepper(t)
	head, tail := &list_head.ListHead{}, &list_head.ListHead{}
	a, b, y, p := &list_head.ListHead{}, &list_head.ListHead{}, &list_head.ListHead{}, &list_head.ListHead{}
	newStepList(t, head, tail, map[string]*list_head.ListHead{"a": a, "b": b, "y": y, "p": p}, "a", "b", "y", "p")

	inA := s.stopAt("prev.waitNoMark", a)
	aDone, _ := goDo(func() error { a.Prev(list_head.WaitNoM()); return nil })
	inA.waitReached(t)
	inB := s.stopAt("prev.waitNoMark", b)
	bDone, _ := goDo(func() error { b.Prev(list_head.WaitNoM()); return nil })
	inB.waitReached(t)
	inA.Release()
	waitClosed(t, aDone, "a.Prev(WaitNoM())")
	inB.Release()
	waitClosed(t, bDone, "b.Prev(WaitNoM())")

	mode := list_head.DefaultModeTraverse.Type()

	marked := s.stopAt("del.marked", p)
	delDone, _ := goDo(func() error { return p.MarkForDelete() })
	marked.waitReached(t)
	got := y.Next()
	marked.Release()
	waitClosed(t, delDone, "p.MarkForDelete")

	if mode != list_head.TravDirect {
		t.Errorf("DefaultModeTraverse after both Prev(WaitNoM()) calls returned = %v, want TravDirect %v", mode, list_head.TravDirect)
	}
	if got != p {
		t.Errorf("y.Next() with no option after both calls returned = %p, want p %p", got, p)
	}
}

// pickI9 returns the first node of pool, not in used and not the last one,
// for which ok holds, and adds it to used. The last node is left out so that
// an 8-byte read one byte past any picked node stays inside pool.
func pickI9(t *testing.T, pool []list_head.ListHead, used map[*list_head.ListHead]bool, ok func(uintptr) bool) *list_head.ListHead {
	t.Helper()
	for i := 0; i < len(pool)-1; i++ {
		n := &pool[i]
		if !used[n] && ok(uintptr(unsafe.Pointer(n))) {
			used[n] = true
			return n
		}
	}
	t.Fatalf("no node in the pool meets the condition")
	return nil
}

// newMarkedLinkListI9 links head p x y tail, taken from one array, and returns
// them. prevWaitNoMark decides whether the link it read is usable by calling
// IsMarked on it, and on p|1 (x.prev while x is being deleted) IsMarked reads
// p's two words one byte off: bit 0 of those reads is bit 8 of p.prev (head)
// and bit 8 of p.next (x). The nodes are picked so that both bits are 0, and
// so that the low byte of x is not 0. The first makes IsMarked(p|1) report
// false, as it does in the stress test whenever both bits happen to be 0;
// the second makes the word prevLoad(p|1) returns, (head>>8) | (x&0xff)<<56,
// a non-canonical address.
func newMarkedLinkListI9(t *testing.T) (names map[*list_head.ListHead]string, head, p, x, y, tail *list_head.ListHead) {
	t.Helper()
	pool := make([]list_head.ListHead, 256)
	used := map[*list_head.ListHead]bool{}
	bit8Clear := func(a uintptr) bool { return a&0x100 == 0 }
	head = pickI9(t, pool, used, bit8Clear)
	x = pickI9(t, pool, used, func(a uintptr) bool { return bit8Clear(a) && a&0xff != 0 })
	anyAddr := func(uintptr) bool { return true }
	p, y, tail = pickI9(t, pool, used, anyAddr), pickI9(t, pool, used, anyAddr), pickI9(t, pool, used, anyAddr)
	names = newStepList(t, head, tail, map[string]*list_head.ListHead{"p": p, "x": x, "y": y}, "p", "x", "y")
	return
}

// Two goroutines, list head p x y tail. prevWaitNoMark looks for marks on the
// node the link points to, and not on the link it read, so a node that is
// being deleted hands its marked link to the caller
// (TestListaWaitNoMarkFromMarkedNode). This test follows that link value
// through Front.
//
//  1. T calls y.Front(). frontCc walks back from y: it reads y.prev = x in the
//     loop condition and in the post statement, then checks the condition
//     with head = x: x.Prev(WaitNoM()) returns p, and x is not marked in the
//     loop body. The post statement calls x.Prev(WaitNoM()) again, and T
//     stops there (the second stop on x; "front.prev" since the walk reads
//     the links itself).
//  2. D calls x.MarkForDelete() and stops at "del.marked".
//  3. T goes on: prevWaitNoMark(x) reads x.prev = p|1 (p with the delete
//     mark) and checks p|1.IsMarked(), which reads memory one byte into p
//     and, with the addresses picked here, reports false. prevWaitNoMark
//     returns p|1 and frontCc sets head = p|1.
//     The loop condition calls (p|1).Prev(WaitNoM()): prevWaitNoMark(p|1)
//     reads the word one byte into p, which is not an address, and calls
//     IsMarked on it. The load faults.
//
// This is the "unexpected fault address" of TestConcurrentAddAndDelete:
// frontCc -> ListPrev(odd address) -> prevWaitNoMark -> IsMarked -> prevLoad.
// When D is released, p is the first node, which Front should return.
func TestReproI9FrontFaultsOnMarkedLink(t *testing.T) {
	useConcurrentMode(t)
	s := newStepper(t)
	names, head, p, x, y, tail := newMarkedLinkListI9(t)

	first := s.stopAt("front.prev", x)
	second := s.stopAt("front.prev", x)
	var got *list_head.ListHead
	var panicked interface{}
	frontDone, _ := goDo(func() error {
		panicked = callRecoverI9(t, func() { got = y.Front() })
		return nil
	})
	first.waitReached(t)
	first.Release()
	second.waitReached(t)

	marked := s.stopAt("del.marked", x)
	delDone, delErr := goDo(func() error { return x.MarkForDelete() })
	marked.waitReached(t)
	second.Release()
	waitClosed(t, frontDone, "y.Front")

	marked.Release()
	waitClosed(t, delDone, "x.MarkForDelete")
	if *delErr != nil {
		t.Fatalf("x.MarkForDelete: %v", *delErr)
	}
	assertLinked(t, names, head, tail, "p", "y")
	if panicked != nil {
		t.Fatalf("y.Front() while x.MarkForDelete stood at del.marked panicked: %v", panicked)
	}
	if got != p {
		t.Errorf("y.Front() = %p, want p %p", got, p)
	}
}

