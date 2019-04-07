package example

import (
	"github.com/kazu/lonacha/list_head"
)

type Sample struct {
	ID     int
	Name   string
	Parent *SampleParent
	list_head.ListHead
}
