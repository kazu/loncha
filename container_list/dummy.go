package list

import (
	"github.com/kazu/loncha/list_head"
)

// ListEntry ... dummy struct.liked-list like a kernel list head.
type ListEntry struct {
	ID   int
	Name string
	list_head.ListHead
}
