//go:build stephook

package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

func TestLenRestartsAfterCurrentNodeIsDeleted(t *testing.T) {
	useConcurrentMode(t)
	head, tail := &list_head.ListHead{}, &list_head.ListHead{}
	a, b := &list_head.ListHead{}, &list_head.ListHead{}
	newStepList(t, head, tail, map[string]*list_head.ListHead{"a": a, "b": b}, "a", "b")
	s := newStepper(t)
	stop := s.stopAt("len.current", b)
	var got int
	done, _ := goDo(func() error {
		got = head.Len()
		return nil
	})
	stop.waitReached(t) // a has already been counted.
	if err := b.MarkForDelete(); err != nil {
		t.Fatal(err)
	}
	stop.Release()
	waitClosed(t, done, "Len after deleting its current node")
	if got != 1 {
		t.Fatalf("Len = %d, want 1", got)
	}
}
