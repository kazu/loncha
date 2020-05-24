package example_test

import (
	"testing"

	"github.com/kazu/loncha"
	"github.com/kazu/loncha/example"
	"github.com/kazu/loncha/list_head"
	"github.com/stretchr/testify/assert"
)

func TestContainerListAdd(t *testing.T) {
	list_head.MODE_CONCURRENT = true
	var list example.Sample
	list.Init()

	hoge := example.Sample{ID: 1, Name: "aaa"}
	hoge.Init()
	list.Add(&hoge)

	hoge2 := example.Sample{ID: 2, Name: "bbb"}
	hoge2.Init()

	hoge.Add(&hoge2)

	assert.Equal(t, hoge.Next().ID, 2)
	assert.Equal(t, hoge.Len(), 2)
	assert.Equal(t, hoge.Next().Len(), 2)

	cnt := 0
	list.Each(func(e *example.Sample) {
		if cnt == 0 {
			assert.Equal(t, 1, e.ID)
		} else {
			assert.Equal(t, 2, e.ID)
		}
		cnt++
	})
	assert.Equal(t, 2, cnt)
}

func TestDelete(t *testing.T) {

	tests := []struct {
		Name    string
		Count   int
		deletes []int
	}{
		{
			Name:    "first middle last delete",
			Count:   10,
			deletes: []int{0, 5, 9},
		},
		{
			Name:    "continus delete",
			Count:   10,
			deletes: []int{4, 5, 6},
		},
		{
			Name:    "continus delete in last",
			Count:   10,
			deletes: []int{3, 4, 5, 8, 9},
		},
		{
			Name:    "continus delete in first",
			Count:   10,
			deletes: []int{0, 1, 2, 4, 5, 6},
		},
		{
			Name:    "all deleted",
			Count:   3,
			deletes: []int{0, 1, 2},
		},
	}

	makeElement := func() *example.Sample {
		e := &example.Sample{}
		e.Init()
		return e
	}

	list_head.MODE_CONCURRENT = true

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var list example.Sample
			list.Init()
			for i := 0; i < test.Count; i++ {
				e := makeElement()
				list.Add(e)

				found := loncha.Contain(&test.deletes, func(idx int) bool {
					return test.deletes[idx] == i
				})
				if found {
					e.Delete()
				}
			}
			if list.Len() != test.Count-len(test.deletes) {
				t.Errorf("missmatch len=%d cnt=%d deletes=%d", list.Len(), test.Count, len(test.deletes))
			}

		})
	}
}
