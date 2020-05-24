package example

import (
	"github.com/kazu/loncha/list_head"
)

type Sample struct {
	ID     int
	Name   string
	Parent *SampleParent
	list_head.ListHead
}
