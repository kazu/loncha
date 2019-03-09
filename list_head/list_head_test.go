package list_head_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kazu/lonacha/list_head"
)

func TestInit(t *testing.T) {
	list := list_head.ListHead{}
	list.Init()

	assert.Equal(t, &list, list.Prev())
	assert.Equal(t, &list, list.Next())

}

func TestAdd(t *testing.T) {
	first := list_head.ListHead{}
	first.Init()

	second := list_head.ListHead{}
	second.Init()

	first.Add(&second)

	assert.Equal(t, first.Prev(), &second)
	assert.Equal(t, first.Next(), &second)
	assert.Equal(t, second.Prev(), &first)
	assert.Equal(t, second.Next(), &first)

}

func TestDelete(t *testing.T) {
	first := list_head.ListHead{}
	first.Init()

	second := list_head.ListHead{}
	second.Init()

	first.Add(&second)

	assert.Equal(t, first.Prev(), &second)
	assert.Equal(t, first.Next(), &second)
	assert.Equal(t, second.Prev(), &first)
	assert.Equal(t, second.Next(), &first)

	second.Delete()

	assert.Equal(t, first.Prev(), &first)
	assert.Equal(t, first.Next(), &first)
	assert.True(t, first.Empty())
	assert.True(t, first.IsLast())

}
