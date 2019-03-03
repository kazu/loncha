package lonacha

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

type Element struct {
	ID   int
	Name string
}

const (
	CREATE_SLICE_MAX int = 1000
)

func MakeSliceSample() (slice []Element) {
	slice = make([]Element, 0, CREATE_SLICE_MAX*2)

	for i := 0; i < CREATE_SLICE_MAX; i++ {
		slice = append(slice,
			Element{
				ID:   int(math.Abs(float64(rand.Intn(CREATE_SLICE_MAX)))),
				Name: fmt.Sprintf("aaa%d", i),
			})
	}
	return
}

type Elements []Element
type PtrElements []*Element

type EqInt map[string]int
type EqString map[string]string

type Eq map[string]interface{}

func (eq EqInt) Func(slice Elements) (funcs []CondFunc) {
	funcs = make([]CondFunc, 0, len(eq))
	for key, _ := range eq {
		switch key {
		case "ID":
			fn := func(i int) bool {
				return slice[i].ID == eq[key]
			}
			funcs = append(funcs, fn)
		}
	}
	return
}

func (eq EqString) Func(slice Elements) (funcs []CondFunc) {
	funcs = make([]CondFunc, 0, len(eq))
	for key, _ := range eq {
		switch key {
		case "Name":
			fn := func(i int) bool {
				return slice[i].Name == eq[key]
			}
			funcs = append(funcs, fn)
		}
	}
	return
}

func (slice Elements) Where(q Eq) Elements {
	eqInt := make(EqInt)
	eqString := make(EqString)
	funcs := make([]CondFunc, 0, len(q))
	for key, value := range q {
		switch key {
		case "ID":
			eqInt[key] = value.(int)
			funcs = append(funcs, eqInt.Func(slice)[0])
		case "Name":
			eqString[key] = value.(string)
			funcs = append(funcs, eqString.Func(slice)[0])
		}
	}
	oslice := &slice
	Filter(oslice, funcs...)
	return *oslice
}

func MakePtrSliceSample() (slice []*Element) {
	slice = make([]*Element, 0, CREATE_SLICE_MAX*2)

	for i := 0; i < CREATE_SLICE_MAX; i++ {
		slice = append(slice,
			&Element{
				ID:   int(math.Abs(float64(rand.Intn(CREATE_SLICE_MAX)))),
				Name: fmt.Sprintf("aaa%d", i),
			})
	}
	return
}

func TestWhere(t *testing.T) {
	slice := Elements(MakeSliceSample())

	nSlice := slice.Where(Eq{"ID": 555})

	assert.True(t, nSlice[0].ID == 555, nSlice)
	assert.True(t, len(nSlice) < 100, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
}

func TestFilter(t *testing.T) {
	nSlice := Elements(MakeSliceSample())

	Filter(&nSlice, func(i int) bool {
		return nSlice[i].ID == 555
	})

	assert.True(t, nSlice[0].ID == 555, nSlice)
	assert.True(t, len(nSlice) < 100, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
}

func TestSelect(t *testing.T) {
	slice := MakeSliceSample()

	ret, err := Select(&slice, func(i int) bool {
		return slice[i].ID < 50
	})
	nSlice, ok := ret.([]Element)

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID < 50, nSlice)
	assert.True(t, ok)
	assert.True(t, len(nSlice) < 100, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
}

func TestPtrSelect(t *testing.T) {
	slice := MakePtrSliceSample()

	ret, err := Select(&slice, func(i int) bool {
		return slice[i].ID < 50
	})
	nSlice, ok := ret.([]*Element)

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID < 50, nSlice)
	assert.True(t, ok)
	assert.True(t, len(nSlice) < 100, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
}

func BenchmarkFilter(b *testing.B) {

	orig := MakeSliceSample()

	b.ResetTimer()
	b.Run("lonacha.Filter", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			Filter(&objs, func(i int) bool {
				return objs[i].ID == 555
			})
		}
	})

	pObjs := MakePtrSliceSample()
	b.ResetTimer()
	b.Run("lonacha.Filter pointer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]*Element, 0, len(pObjs))
			copy(objs, pObjs)
			b.StartTimer()
			Filter(&objs, func(i int) bool {
				return objs[i].ID == 555
			})
		}
	})

	b.ResetTimer()
	b.Run("hand Filter pointer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]*Element, len(orig))
			copy(objs, pObjs)
			b.StartTimer()
			result := make([]*Element, 0, len(orig))
			for idx, _ := range objs {
				if objs[idx].ID == 555 {
					result = append(result, objs[idx])
				}
			}
		}
	})

}

func BenchmarkSelect(b *testing.B) {
	orig := MakeSliceSample()

	b.ResetTimer()
	b.Run("lonacha.Select", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			Select(&objs, func(i int) bool {
				return objs[i].ID == 555
			})
		}
	})

	b.ResetTimer()
	b.Run("lonacha.FilterAndCopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			Filter(&objs, func(i int) bool {
				return objs[i].ID == 555
			})
			newObjs := make([]Element, len(orig))
			copy(newObjs, objs)
		}
	})

	b.ResetTimer()
	b.Run("hand Select", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			result := make([]Element, len(orig))
			for idx, _ := range objs {
				if objs[idx].ID == 555 {
					result = append(result, objs[idx])
				}
			}
		}
	})

}
