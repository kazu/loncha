//go:build stephook

package list_head_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// stepper stops goroutines at the step points of lista_encabezado, so that a
// test replays a concurrent interleaving one step at a time.
type stepper struct {
	mu    sync.Mutex
	stops []*stepStop
}

type stepStop struct {
	point   string
	node    *list_head.ListHead
	used    bool
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

func newStepper(t *testing.T) *stepper {
	t.Helper()
	s := &stepper{}
	list_head.SetStepHook(s.at)
	t.Cleanup(func() {
		list_head.SetStepHook(nil)
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, st := range s.stops {
			st.Release()
		}
	})
	return s
}

// stopAt stops the first goroutine that reaches point with node as its first
// argument, until Release is called.
func (s *stepper) stopAt(point string, node *list_head.ListHead) *stepStop {
	st := &stepStop{point: point, node: node, reached: make(chan struct{}), release: make(chan struct{})}
	s.mu.Lock()
	s.stops = append(s.stops, st)
	s.mu.Unlock()
	return st
}

func (s *stepper) at(point string, a, b, c *list_head.ListHead) {
	s.mu.Lock()
	var st *stepStop
	for _, x := range s.stops {
		if !x.used && x.point == point && x.node == a {
			x.used = true
			st = x
			break
		}
	}
	s.mu.Unlock()
	if st == nil {
		return
	}
	close(st.reached)
	<-st.release
}

func (st *stepStop) Release() {
	st.once.Do(func() { close(st.release) })
}

// waitReached fails the test unless a goroutine stops at st.
func (st *stepStop) waitReached(t *testing.T) {
	t.Helper()
	waitClosed(t, st.reached, st.point)
}

func waitClosed(t *testing.T, c <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-c:
	case <-time.After(10 * time.Second):
		t.Fatalf("%s did not happen", what)
	}
}

// goDo runs fn in a new goroutine and returns a channel closed when fn
// returns, and a pointer to the error fn returned.
func goDo(fn func() error) (<-chan struct{}, *error) {
	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		err = fn()
	}()
	return done, &err
}

// useConcurrentMode turns on MODE_CONCURRENT for the test, as InsertBefore
// links nodes with CAS only in that mode.
func useConcurrentMode(t *testing.T) {
	t.Helper()
	old := list_head.MODE_CONCURRENT
	list_head.MODE_CONCURRENT = true
	t.Cleanup(func() { list_head.MODE_CONCURRENT = old })
}

// newStepList links the nodes in order between head and tail and returns the
// names of all of them, head and tail included.
func newStepList(t *testing.T, head, tail *list_head.ListHead, nodes map[string]*list_head.ListHead, order ...string) map[*list_head.ListHead]string {
	t.Helper()
	list_head.InitAsEmpty(head, tail)
	names := map[*list_head.ListHead]string{head: "head", tail: "tail"}
	for _, name := range order {
		n := nodes[name]
		n.Init()
		if _, err := tail.InsertBefore(n); err != nil {
			t.Fatal(err)
		}
		names[n] = name
	}
	return names
}

// walk follows the links from head to tail and back, and returns the names of
// the nodes between them in each direction. It stops with an error at a link
// that is not one of the named nodes, so that it never dereferences a marked
// or foreign pointer.
func walk(names map[*list_head.ListHead]string, head, tail *list_head.ListHead) (fwd, bwd []string, err error) {
	for cur := head.DirectNext(); cur != tail; cur = cur.DirectNext() {
		if _, ok := names[cur]; !ok || len(fwd) > len(names) {
			return fwd, bwd, fmt.Errorf("forward walk reached %p after %q", cur, fwd)
		}
		fwd = append(fwd, names[cur])
	}
	for cur := tail.DirectPrev(); cur != head; cur = cur.DirectPrev() {
		if _, ok := names[cur]; !ok || len(bwd) > len(names) {
			return fwd, bwd, fmt.Errorf("backward walk reached %p after %q", cur, bwd)
		}
		bwd = append([]string{names[cur]}, bwd...)
	}
	return fwd, bwd, nil
}

// assertLinked checks that the list from head to tail holds exactly the nodes
// named in want, walking forward and walking backward.
func assertLinked(t *testing.T, names map[*list_head.ListHead]string, head, tail *list_head.ListHead, want ...string) {
	t.Helper()
	fwd, bwd, err := walk(names, head, tail)
	if err != nil {
		t.Errorf("%v; want %q", err, want)
		return
	}
	for _, got := range [][]string{fwd, bwd} {
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("forward %q, backward %q, want %q", fwd, bwd, want)
			return
		}
	}
}
