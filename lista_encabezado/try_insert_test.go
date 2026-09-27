package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

// newTryInsertList links a and b in this order between head and tail.
func newTryInsertList(t *testing.T) (head, a, b, tail *list_head.ListHead) {
	t.Helper()
	list_head.MODE_CONCURRENT = true
	e := make([]list_head.ListHead, 4)
	head, a, b, tail = &e[0], &e[1], &e[2], &e[3]
	list_head.InitAsEmpty(head, tail)
	for _, n := range []*list_head.ListHead{a, b} {
		n.Init()
		if _, err := tail.InsertBefore(n); err != nil {
			t.Fatal(err)
		}
	}
	return
}

func TestTryInsertBeforeLinksWhenAccepted(t *testing.T) {
	head, a, b, tail := newTryInsertList(t)
	n := &list_head.ListHead{}
	n.Init()
	var got *list_head.ListHead
	if err := b.TryInsertBefore(n, func(prev *list_head.ListHead) bool { got = prev; return true }); err != nil {
		t.Fatalf("TryInsertBefore: %v", err)
	}
	if got != a {
		t.Errorf("accept got %p, want a %p", got, a)
	}
	if a.DirectNext() != n || n.DirectNext() != b || b.DirectPrev() != n || n.DirectPrev() != a {
		t.Errorf("n is not linked between a and b")
	}
	_, _ = head, tail
}

func TestTryInsertBeforeRejected(t *testing.T) {
	_, a, b, _ := newTryInsertList(t)
	n := &list_head.ListHead{}
	n.Init()
	if err := b.TryInsertBefore(n, func(*list_head.ListHead) bool { return false }); err != list_head.ErrNotAppend {
		t.Fatalf("TryInsertBefore rejected by accept = %v, want ErrNotAppend", err)
	}
	if a.DirectNext() != b || b.DirectPrev() != a {
		t.Errorf("a rejected insert changed the links of a and b")
	}
}

func TestTryInsertBeforeMarkedNode(t *testing.T) {
	_, _, b, _ := newTryInsertList(t)
	if err := b.MarkForDelete(); err != nil {
		t.Fatal(err)
	}
	n := &list_head.ListHead{}
	n.Init()
	if err := b.TryInsertBefore(n, func(*list_head.ListHead) bool { return true }); err != list_head.ErrMarked {
		t.Fatalf("TryInsertBefore before a deleted node = %v, want ErrMarked", err)
	}
}
