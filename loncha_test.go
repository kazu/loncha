package loncha

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/thoas/go-funk"

	"github.com/stretchr/testify/assert"
)

type Element struct {
	ID   int
	Name string
}

const (
	CREATE_SLICE_MAX int = 10000
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
	OldFilter(oslice, funcs...)
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

func TestFind(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	id := nSlice[50].ID
	data, err := Find(&nSlice, func(i int) bool {
		return nSlice[i].ID == id
	})

	assert.NoError(t, err)
	elm := data.(Element)
	assert.True(t, elm.ID == id, elm)

	nSlice = MakeSliceSample()
	id = nSlice[50].ID
	data, err = Find(nSlice, func(i int) bool {
		return nSlice[i].ID == id
	})

	assert.NoError(t, err)
	elm = data.(Element)
	assert.True(t, elm.ID == id, elm)

}

func TestLastIndexOf(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	id := nSlice[50].ID
	i, err := LastIndexOf(nSlice, func(i int) bool {
		return nSlice[i].ID == id
	})

	assert.NoError(t, err)
	assert.True(t, nSlice[i].ID == id, nSlice[i])

	nSlice = MakeSliceSample()
	id = nSlice[50].ID
	i, err = LastIndexOf(nSlice, func(i int) bool {
		return nSlice[i].ID == id
	})

	assert.NoError(t, err)
	assert.True(t, nSlice[i].ID == id, nSlice[i])

}

func TestFilter(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	id := nSlice[50].ID
	OldFilter(&nSlice, func(i int) bool {
		return nSlice[i].ID == id
	})

	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
}

func TestFilter2(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	id := nSlice[50].ID
	var err error
	nSlice, err = Filter(nSlice, nil,
		Cond2[FilterOpt[Element]](func(obj *Element) bool {
			return obj.ID == id
		}))

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	nSlice = Elements(MakeSliceSample())
	id = nSlice[50].ID
	nSlice, err = Filter(nSlice, nil,
		FilterVersion[FilterOpt[Element]](4),
		Cond2[FilterOpt[Element]](func(obj *Element) bool {
			return obj.ID == id || obj.ID == id+100
		}))

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	nSlice = Elements(MakeSliceSample())
	id = nSlice[50].ID
	expect := nSlice[50]
	nSlice, err = Filter(nSlice, nil,
		FilterVersion[FilterOpt[Element]](3),
		Equal[FilterOpt[Element]](expect))

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	nSlice = Elements(MakeSliceSample())
	id = nSlice[50].ID
	nSlice, err = Filter(nSlice,
		func(obj *Element) bool {
			return obj.ID == id || obj.ID == id+100
		})

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	nSlice = Elements(MakeSliceSample())
	id = nSlice[50].ID
	nSlice, err = Filter(nSlice, nil,
		FilterVersion[FilterOpt[Element]](3),
		Cond2[FilterOpt[Element]](func(obj *Element) bool {
			return obj.ID == id || obj.ID == id+100
		}))

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	nSlice = Elements(MakeSliceSample())
	id = nSlice[50].ID
	nSlice = Filterable(
		func(obj *Element) bool {
			return obj.ID == id || obj.ID == id+100
		})(nSlice)

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID == id, nSlice)
	assert.True(t, len(nSlice) < CREATE_SLICE_MAX, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

}

func TestDelete(t *testing.T) {

	nSlice := Elements(MakeSliceSample())

	beforeSlice := make(Elements, len(nSlice))
	copy(beforeSlice[:len(nSlice)], nSlice)
	afterSlice := make(Elements, len(nSlice))

	size := len(nSlice)
	id := nSlice[100].ID
	Delete(&nSlice, func(i int) bool {
		return nSlice[i].ID == id
	})

	assert.True(t, nSlice[0].ID != 555, nSlice)
	assert.True(t, len(nSlice) < size, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	afterSlice = afterSlice[:len(nSlice)]
	copy(afterSlice[:len(nSlice)], nSlice)
	nSlice = nSlice[:len(beforeSlice)]
	copy(nSlice[:len(beforeSlice)], beforeSlice)

	size = len(nSlice)
	deleteCond := func(e *Element) bool {
		return e.ID == id
	}
	nSlice = Deletable(deleteCond)(nSlice)

	assert.Equal(t, afterSlice, nSlice)

}

func TestUniq(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	nSlice = append(nSlice, Element{ID: nSlice[0].ID})
	size := len(nSlice)

	fn := func(i int) interface{} { return i }
	assert.NotEqual(t, fn(1), fn(2))
	Uniq(&nSlice, func(i int) interface{} {
		return nSlice[i].ID
	})

	assert.True(t, len(nSlice) < size, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
}

func TestUniq2(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	nSlice = append(nSlice, Element{ID: nSlice[0].ID})
	size := len(nSlice)
	nSlice2 := make([]Element, len(nSlice))
	copy(nSlice2, nSlice)

	Uniq2(&nSlice, func(i, j int) bool {
		return nSlice[i].ID == nSlice[j].ID
	})
	Uniq(&nSlice2, func(i int) interface{} {
		return nSlice2[i].ID
	})

	assert.True(t, len(nSlice) < size, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
	assert.Equal(t, len(nSlice), len(nSlice2))

}

func TestUniqWithSort(t *testing.T) {
	nSlice := Elements(MakeSliceSample())
	nSlice = append(nSlice, Element{ID: nSlice[0].ID})
	//size := len(nSlice)
	nSlice2 := make([]Element, len(nSlice))
	copy(nSlice2, nSlice)

	UniqWithSort(&nSlice, func(i, j int) bool {
		return nSlice[i].ID < nSlice[j].ID
	})

	//assert.True(t, len(nSlice) < size, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))
	assert.Equal(t, len(nSlice), len(nSlice2))

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

	slice = MakeSliceSample()

	ret, err = Select(slice, func(i int) bool {
		return slice[i].ID < 50
	})
	nSlice, ok = ret.([]Element)

	assert.NoError(t, err)
	assert.True(t, nSlice[0].ID < 50, nSlice)
	assert.True(t, ok)
	assert.True(t, len(nSlice) < 100, len(nSlice))
	t.Logf("nSlice.len=%d cap=%d\n", len(nSlice), cap(nSlice))

	slice = MakeSliceSample()

	islessID := func(id int) func(e *Element) bool {
		return func(e *Element) bool {
			return e.ID < id
		}
	}

	nSlice = Selectable(islessID(50))(slice)

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

func TestShuffle(t *testing.T) {
	slice := MakeSliceSample()
	a := slice[0]

	err := Shuffle(slice, 2)

	assert.NoError(t, err)
	assert.NotEqual(t, slice[0].ID, a.ID)
}

func TestReverse(t *testing.T) {
	slice := []int{1, 4, 2, 6, 4, 6}
	Reverse(slice)

	assert.Equal(t, []int{6, 4, 6, 2, 4, 1}, slice)
}

func TestIntersect(t *testing.T) {
	slice1 := []int{1, 4, 2, 6, 4, 6}
	slice2 := []int{2, 5, 9, 6, 4}

	result := Intersect(slice1, slice2)

	assert.Equal(t, []int{2, 6, 4}, result)
}

func TestIntersectSSorted(t *testing.T) {
	slice1 := []int{6, 4, 2, 1}
	slice2 := []int{9, 6, 5, 3, 2}
	Reverse(slice1)
	Reverse(slice2)

	result := IntersectSorted(slice1, slice2, func(s []int, i int) int {
		return s[i]
	})

	assert.Equal(t, []int{2, 6}, result)
}

func TestSub(t *testing.T) {
	slice1 := []int{6, 4, 2, 1}
	slice2 := []int{9, 6, 5, 3, 2}
	Reverse(slice1)
	Reverse(slice2)

	result := Sub(slice1, slice2)

	assert.Equal(t, []int{1, 4}, result)
}

func TestSubSorted(t *testing.T) {
	slice1 := []int{10, 6, 4, 2}
	slice2 := []int{9, 6, 5, 3, 2, 1}
	Reverse(slice1)
	Reverse(slice2)

	result := SubSorted(slice1, slice2, func(s []int, i int) int {
		return s[i]
	})

	assert.Equal(t, []int{4, 10}, result)
}

type V4sum struct {
	A int
}

func TestInject(t *testing.T) {
	slice1 := []int{10, 6, 4, 2}

	sum1 := Inject(slice1, func(sum int, t int) int {
		return sum + t
	})
	assert.Equal(t, 22, sum1)

	sum := Reduce(slice1, func(sum *int, t int) *int {
		if sum == nil {
			sum = new(int)
			*sum = 0
		}
		v := *sum + t
		return &v
	})
	assert.Equal(t, 22, *sum)

	sum4 := Sum(slice1)

	assert.Equal(t, 22, sum4)

	sum2 := Reducable(func(sum *int, t int) *int {
		if sum == nil {
			sum = new(int)
			*sum = 0
		}
		v := *sum + t
		return &v
	})(slice1)
	assert.Equal(t, *sum, *sum2)

	d := int(0)
	_ = d

	sum3 := Reducable(func(sum *int, t int) *int {
		if sum == nil {
			sum = new(int)
			*sum = 0
		}
		v := *sum + t
		return &v
	}, Default(&d))(slice1)
	assert.Equal(t, *sum, *sum3)

	slice2 := []V4sum{
		{1}, {2},
	}

	sumFns := SumWithFn(slice2, func(a V4sum) int { return a.A })
	assert.Equal(t, sumFns, 3)

}

func TestGettable(t *testing.T) {

	slice1 := []int{10, 6, 4, 2}

	r := Gettable(func(v *int) bool { return *v == 4 })(slice1)

	assert.Equal(t, 4, r)

}

func TestConv(t *testing.T) {
	slice1 := []int{10, 6, 4, 2}

	int64s := Convertable(
		func(i int) (int64, bool) {
			return int64(100 + i), false
		})(slice1)

	assert.Equal(t, slice1[0]+100, int(int64s[0]))

	slice1 = []int{10, 6, 4, 2}

	int64s = ConvertableKeep(
		func(i int) int64 {
			return int64(100 + i)
		})(slice1...)

	assert.Equal(t, slice1[0]+100, int(int64s[0]))

}

func has[T comparable](a T) func(e *T) bool {
	return func(e *T) bool {
		return *e == a
	}

}

func TestContain(t *testing.T) {
	slice1 := []int{10, 6, 4, 2}

	assert.True(t, Containable(has(6))(slice1))
	assert.False(t, Containable(has(11))(slice1))

	assert.True(t,
		Contain(slice1, func(i int) bool { return slice1[i] == 6 }))

}

func TestEvery(t *testing.T) {
	slice1 := []int{10, 6, 4, 2}

	assert.True(t,
		Every(func(v *int) bool {
			*v = +1
			return true
		})(slice1...))

	assert.False(t,
		Every(func(v *int) bool {
			*v++
			if *v == 5 {
				return false
			}
			return true
		})(slice1...))
	assert.True(t,
		EveryWithIndex(func(i int, v *int) bool {
			*v = +1
			return true
		})(slice1...))

}

func Sort[T Ordered](s []T) []T {

	sort.Slice(s, func(i, j int) bool {
		return s[i] <= s[j]
	})

	return s
}

func TestMap(t *testing.T) {

	m := map[int]int{
		1: 20,
		4: 30,
	}
	assert.Equal(t, []int{1, 4}, Sort(Keys(m)))
	assert.Nil(t, Keys[int, int](nil))
	assert.Equal(t, []int{20, 30}, Sort(Values(m)))
	assert.Nil(t, Values[int, int](nil))

	nM := SelectMap(m, func(k, v int) (int, int, bool) {
		if k+v == 34 {
			return k, v, true
		}
		return k + 10, v + 100, false
	})

	assert.Equal(t, 120, nM[11])

}

func TestZipper(t *testing.T) {

	a := []string{"bob", "hoge", "one", "home"}
	b := []int{0, 1, 2}

	m := Zipper(ToMap[string, int], map[string]int{})(a, b)

	assert.Equal(t, 0, m["bob"])
	assert.Equal(t, 2, m["one"])
	assert.Equal(t, 0, m["home"])

}

// BenchmarkFilter/loncha.Filter-16         	     100	     89142 ns/op	   82119 B/op	       4 allocs/op
// BenchmarkFilter/loncha.Filter_pointer-16 	     100	       201 ns/op	       0 B/op	       0 allocs/op
// BenchmarkFilter/hand_Filter_pointer-16   	     100	     24432 ns/op	   81921 B/op	       1 allocs/op
// BenchmarkFilter/go-funk.Filter-16        	     100	   2370492 ns/op	  640135 B/op	   20004 allocs/op
// BenchmarkFilter/go-funk.Filter_pointer-16         100	      1048 ns/op	      64 B/op	       2 allocs/op
func BenchmarkFilter(b *testing.B) {

	orig := MakeSliceSample()

	b.ResetTimer()
	b.Run("loncha.Filter ", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			OldFilter(&objs, func(i int) bool {
				return objs[i].ID == 555
			})
		}
	})

	b.ResetTimer()
	b.Run("loncha.oFilter2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			Filter(objs,
				nil,
				Cond[FilterOpt[Element]](func(i int) bool {
					return objs[i].ID == 555
				}))
		}
	})

	b.ResetTimer()
	b.Run("loncha.Filter2 ", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			objs, _ = Filter(objs, nil,
				Cond2[FilterOpt[Element]](func(obj *Element) bool {
					return obj.ID == 555
				}))
		}
	})
	b.ResetTimer()
	b.Run("loncha.Filter2.3 ", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			objs, _ = Filter(objs, nil,
				FilterVersion[FilterOpt[Element]](3),
				Cond2[FilterOpt[Element]](func(obj *Element) bool {
					return obj.ID == 555
				}))
		}
	})
	b.ResetTimer()
	b.Run("loncha.Filter2.4 ", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			objs, _ = Filter(objs, nil,
				FilterVersion[FilterOpt[Element]](4),
				Cond2[FilterOpt[Element]](func(obj *Element) bool {
					return obj.ID == 555
				}))
		}
	})

	b.ResetTimer()
	b.Run("loncha.Filterable ", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			objs = Filterable(func(obj *Element) bool {
				return obj.ID == 555
			})(objs)

		}
	})

	pObjs := MakePtrSliceSample()
	b.ResetTimer()
	b.Run("loncha.Filter pointer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]*Element, 0, len(pObjs))
			copy(objs, pObjs)
			b.StartTimer()
			OldFilter(&objs, func(i int) bool {
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

	b.ResetTimer()
	b.Run("go-funk.Filter", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			funk.Filter(objs, func(e Element) bool {
				return e.ID == 555
			})
		}
	})

	b.ResetTimer()
	b.Run("go-funk.Filter pointer", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]*Element, 0, len(pObjs))
			copy(objs, pObjs)
			b.StartTimer()
			funk.Filter(objs, func(e *Element) bool {
				return e.ID == 555
			})
		}
	})
}

// BenchmarkUniq/loncha.Uniq-16         	    			1000	    997543 ns/op	  548480 B/op	   16324 allocs/op
// BenchmarkUniq/loncha.UniqWithSort-16 	    			1000	   2237924 ns/op	     256 B/op	       7 allocs/op
// BenchmarkUniq/loncha.UniqWithSort(sort)-16         	    1000	    260283 ns/op	     144 B/op	       4 allocs/op
// BenchmarkUniq/hand_Uniq-16                          	    1000	    427765 ns/op	  442642 B/op	       8 allocs/op
// BenchmarkUniq/hand_Uniq_iface-16                    	    1000	    808895 ns/op	  632225 B/op	    6322 allocs/op
// BenchmarkUniq/go-funk.Uniq-16                       	    1000	   1708396 ns/op	  655968 B/op	   10004 allocs/op
func BenchmarkUniq(b *testing.B) {

	orig := MakeSliceSample()

	b.ResetTimer()
	b.Run("loncha.UniqWithSort(before sort)", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			sort.Slice(objs, func(i, j int) bool {
				return objs[i].ID < objs[j].ID
			})
			b.StartTimer()
			UniqWithSort(&objs, func(i, j int) bool {
				return objs[i].ID < objs[j].ID
			})
		}
	})

	b.ResetTimer()
	b.Run("loncha.UniqWithSort", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			UniqWithSort(&objs, func(i, j int) bool {
				return objs[i].ID < objs[j].ID
			})
		}
	})

	b.ResetTimer()
	b.Run("loncha.Uniq", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			Uniq(&objs, func(i int) interface{} {
				return objs[i].ID
			})
		}
	})

	b.Run("loncha.Uniq2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			Uniq2(&objs, func(i, j int) bool {
				return objs[i].ID == objs[j].ID
			})
		}
	})

	b.ResetTimer()
	b.Run("hand Uniq", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			exists := make(map[int]bool, len(objs))
			result := make([]Element, 0, len(orig))
			for idx, _ := range objs {
				if !exists[objs[idx].ID] {
					exists[objs[idx].ID] = true
					result = append(result, orig[idx])
				}
			}
		}
	})

	b.ResetTimer()
	b.Run("hand Uniq iface", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			exists := make(map[interface{}]bool, len(objs))
			result := make([]Element, 0, len(orig))
			for idx, _ := range objs {
				if !exists[objs[idx].ID] {
					exists[objs[idx].ID] = true
					result = append(result, orig[idx])
				}
			}
		}
	})

	b.ResetTimer()
	b.Run("go-funk.Uniq", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			funk.Uniq(objs)
		}
	})
}

func BenchmarkSelect(b *testing.B) {
	orig := MakeSliceSample()

	b.ResetTimer()
	b.Run("loncha.Select", func(b *testing.B) {
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
	b.Run("loncha.FilterAndCopy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			objs := make([]Element, len(orig))
			copy(objs, orig)
			b.StartTimer()
			OldFilter(&objs, func(i int) bool {
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

type TestInterface interface {
	Inc() int
	Name() string
}

type TestObject struct {
	Cnt  int
	name string
}

func (o TestObject) Inc() int {
	o.Cnt++
	return o.Cnt
}

func (o TestObject) Name() string {
	return o.name
}

func BenchmarkCall(b *testing.B) {

	b.ResetTimer()
	b.Run("struct call", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			object := TestObject{Cnt: 0, name: "Test"}
			b.StartTimer()
			for j := 0; j < 100000; j++ {
				object.Inc()
			}
		}
	})

	b.ResetTimer()
	b.Run("interface call", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			object := TestInterface(TestObject{Cnt: 0, name: "Test"})
			b.StartTimer()
			for j := 0; j < 100000; j++ {
				object.Inc()
			}
		}
	})
}

func (list PtrElements) Len() int           { return len(list) }
func (list PtrElements) Swap(i, j int)      { list[i], list[j] = list[j], list[i] }
func (list PtrElements) Less(i, j int) bool { return list[i].ID < list[j].ID }

// BenchmarkSortPtr/sort.Sort-16         	    1000	   1712284 ns/op	      32 B/op	       1 allocs/op
// BenchmarkSortPtr/sort.Slice-16        	    2000	   1170132 ns/op	      64 B/op	       2 allocs/op

func BenchmarkSortPtr(b *testing.B) {
	orig := MakePtrSliceSample()

	b.ResetTimer()
	b.Run("sort.Sort", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			data := make(PtrElements, len(orig))
			copy(data, orig)
			b.StartTimer()
			sort.Sort(data)
		}
	})

	b.ResetTimer()
	b.Run("sort.Slice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			data := make([]*Element, len(orig))
			copy(data, orig)
			b.StartTimer()
			sort.Slice(data, func(i, j int) bool { return data[i].ID < data[j].ID })
		}
	})
}

func (list Elements) Len() int           { return len(list) }
func (list Elements) Swap(i, j int)      { list[i], list[j] = list[j], list[i] }
func (list Elements) Less(i, j int) bool { return list[i].ID < list[j].ID }

// BenchmarkSort/sort.Sort-16         	    1000	   1648947 ns/op	      34 B/op	       1 allocs/op
// BenchmarkSort/sort.Slice-16        	    1000	   1973036 ns/op	     112 B/op	       3 allocs/op

func BenchmarkSort(b *testing.B) {
	orig := MakeSliceSample()

	b.ResetTimer()
	b.Run("sort.Sort", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			data := make(Elements, len(orig))
			copy(data, orig)
			b.StartTimer()
			sort.Sort(data)
		}
	})

	b.ResetTimer()
	b.Run("sort.Slice", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			data := make([]Element, len(orig))
			copy(data, orig)
			b.StartTimer()
			sort.Slice(data, func(i, j int) bool { return data[i].ID < data[j].ID })
		}
	})
}
