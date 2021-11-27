package list_head_test

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
	"github.com/stretchr/testify/assert"
)

func Test_Map(t *testing.T) {
	const concurretRoutine = 10
	const operationCnt = 10
	const percentWrite = 50

	var wg sync.WaitGroup
	m := list_head.Map{}

	wg.Add(1)

	for i := 0; i < concurretRoutine; i++ {
		wg.Add(1)
		go func(i int, wg *sync.WaitGroup) {

			for j := 0; j < operationCnt; j++ {
				m.Set(fmt.Sprintf("%d %d", i, j), &list_head.ListHead{})
			}

			for j := 0; j < operationCnt; j++ {
				_, ok := m.Get(fmt.Sprintf("%d %d", i, j))
				assert.True(t, ok)
			}

			wg.Done()
		}(i, &wg)

	}
	wg.Done()
	wg.Wait()
}

func runBnech(b *testing.B, m list_head.MapGetSet, concurretRoutine, operationCnt int, pctWrites uint64) {

	b.ReportAllocs()
	// var wg sync.WaitGroup
	// for i := 0; i < concurretRoutine; i++ {
	// 	wg.Add(1)
	// 	go func(i int, wg *sync.WaitGroup) {

	// 		for j := 0; j < operationCnt; j++ {
	// 			m.Set(fmt.Sprintf("%d %d", i, j), &list_head.ListHead{})
	// 		}

	// 		for j := 0; j < operationCnt; j++ {
	// 			m.Get(fmt.Sprintf("%d %d", i, j))
	// 		}

	// 		wg.Done()
	// 	}(i, &wg)
	// }

	// wg.Wait()
	size := operationCnt
	mask := size - 1
	rc := uint64(0)

	for j := 0; j < operationCnt; j++ {
		m.Set(fmt.Sprintf("%d", j), &list_head.ListHead{})
	}
	if rmap, ok := m.(*list_head.RMap); ok {
		_ = rmap
		//rmap.ValidateDirty()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		index := rand.Int() & mask
		mc := atomic.AddUint64(&rc, 1)

		if pctWrites*mc/100 != pctWrites*(mc-1)/100 {
			for pb.Next() {
				m.Set(fmt.Sprintf("%d", index&mask), &list_head.ListHead{})
				index = index + 1
			}
		} else {
			for pb.Next() {
				ok := false
				_, ok = m.Get(fmt.Sprintf("%d", index&mask))
				if !ok {
					_, ok = m.Get(fmt.Sprintf("%d", index&mask))
					fmt.Printf("fail")
				}
				index = index + 1
			}
		}
	})

}

type syncMap struct {
	m sync.Map
}

func (m syncMap) Get(k string) (v *list_head.ListHead, ok bool) {

	ov, ok := m.m.Load(k)
	v, ok = ov.(*list_head.ListHead)
	return
}

func (m syncMap) Set(k string, v *list_head.ListHead) (ok bool) {

	m.m.Store(k, v)
	return true
}
func Benchmark_HMap_forProfile(b *testing.B) {
	newShard := func(fn func(int) list_head.MapGetSet) list_head.MapGetSet {
		s := &list_head.ShardMap{}
		s.InitByFn(fn)
		return s
	}
	_ = newShard

	benchmarks := []struct {
		name       string
		concurrent int
		cnt        int
		percent    int
		buckets    int
		mode       list_head.SearchMode
		mapInf     list_head.MapGetSet
	}{
		//{"HMap_combine    ", 100, 100000, 0, 0x010, list_head.CombineSearch, list_head.NewHMap()},
		{"HMap_combine2    ", 100, 100000, 0, 0x010, list_head.CombineSearch2, newWrapHMap(list_head.NewHMap())},
	}

	for _, bm := range benchmarks {
		b.Run(fmt.Sprintf("%s w/%2d bucket=%3d", bm.name, bm.percent, bm.buckets), func(b *testing.B) {
			if whmap, ok := bm.mapInf.(*WrapHMap); ok {
				list_head.MaxPefBucket(bm.buckets)(whmap.base)
				list_head.BucketMode(bm.mode)(whmap.base)
			}
			runBnech(b, bm.mapInf, bm.concurrent, bm.cnt, uint64(bm.percent))
		})
	}
}

type WrapHMap struct {
	base *list_head.HMap
}

func (w *WrapHMap) Set(k string, v *list_head.ListHead) bool {

	return w.base.Set(&list_head.SampleItem{K: k, V: v})
}

func (w *WrapHMap) Get(k string) (v *list_head.ListHead, ok bool) {
	result, ok := w.base.Get(k)
	if !ok || result == nil {
		return nil, ok
	}
	v = result.Value().(*list_head.ListHead)
	return
}

func newWrapHMap(hmap *list_head.HMap) *WrapHMap {
	list_head.ItemFn(func() list_head.MapItem {
		return &list_head.EmptySampleHMapEntry
	})(hmap)

	return &WrapHMap{base: hmap}
}

func Benchmark_HMap(b *testing.B) {

	newShard := func(fn func(int) list_head.MapGetSet) list_head.MapGetSet {
		s := &list_head.ShardMap{}
		s.InitByFn(fn)
		return s
	}
	_ = newShard

	benchmarks := []struct {
		name       string
		concurrent int
		cnt        int
		percent    int
		buckets    int
		mode       list_head.SearchMode
		mapInf     list_head.MapGetSet
	}{
		// {"HMap               ", 100, 100000, 0, 0x020, list_head.NewHMap()},
		// {"HMap               ", 100, 100000, 0, 0x040, list_head.NewHMap()},
		// {"HMap               ", 100, 100000, 0, 0x080, list_head.NewHMap()},
		// {"HMap               ", 100, 100000, 0, 0x100, list_head.NewHMap()},

		{"HMap               ", 100, 100000, 0, 0x200, list_head.LenearSearchForBucket, newWrapHMap(list_head.NewHMap())},

		// // {"HMap               ", 100, 100000, 0, 0x258, list_head.LenearSearchForBucket, list_head.NewHMap()},
		// // {"HMap               ", 100, 100000, 0, 0x400, list_head.LenearSearchForBucket, list_head.NewHMap()},

		// // {"HMap_nestsearch    ", 100, 100000, 0, 0x020, list_head.NestedSearchForBucket, list_head.NewHMap()},
		// // {"HMap_nestsearch    ", 100, 100000, 0, 0x040, list_head.NestedSearchForBucket, list_head.NewHMap()},
		// // {"HMap_nestsearch    ", 100, 100000, 0, 0x080, list_head.NestedSearchForBucket, list_head.NewHMap()},

		// {"HMap_nestsearch    ", 100, 100000, 0, 0x010, list_head.NestedSearchForBucket, list_head.NewHMap()},

		{"HMap_nestsearch    ", 100, 100000, 0, 0x020, list_head.NestedSearchForBucket, newWrapHMap(list_head.NewHMap())},
		{"HMap_combine       ", 100, 100000, 0, 0x020, list_head.CombineSearch, newWrapHMap(list_head.NewHMap())},
		{"HMap_combine       ", 100, 100000, 0, 0x010, list_head.CombineSearch, newWrapHMap(list_head.NewHMap())},
		{"HMap_combine2      ", 100, 100000, 0, 0x010, list_head.CombineSearch2, newWrapHMap(list_head.NewHMap())},

		// {"HMap_nestsearch    ", 100, 100000, 0, 0x400, list_head.NestedSearchForBucket, list_head.NewHMap()},

		// {"HMap_nestsearch    ", 100, 100000, 0, 0x020, list_head.NoItemSearchForBucket, list_head.NewHMap()},
		// {"HMap_nestsearch    ", 100, 100000, 0, 0x020, list_head.FalsesSearchForBucket, list_head.NewHMap()},

		// {"HMap               ", 100, 200000, 0, 0x200, list_head.NewHMap()},
		// {"HMap               ", 100, 200000, 0, 0x300, list_head.NewHMap()},

		//		{"HMap               ", 100, 100000, 50, list_head.NewHMap()},
	}

	for _, bm := range benchmarks {
		b.Run(fmt.Sprintf("%s w/%2d bucket=%3d", bm.name, bm.percent, bm.buckets), func(b *testing.B) {
			if whmap, ok := bm.mapInf.(*WrapHMap); ok {
				list_head.MaxPefBucket(bm.buckets)(whmap.base)
				list_head.BucketMode(bm.mode)(whmap.base)
			}
			runBnech(b, bm.mapInf, bm.concurrent, bm.cnt, uint64(bm.percent))
		})
	}

}

func Benchmark_Map(b *testing.B) {

	newShard := func(fn func(int) list_head.MapGetSet) list_head.MapGetSet {
		s := &list_head.ShardMap{}
		s.InitByFn(fn)
		return s
	}
	_ = newShard

	benchmarks := []struct {
		name       string
		concurrent int
		cnt        int
		percent    int
		mapInf     list_head.MapGetSet
	}{

		// {"WithLock           ", 100, 100000, 0, &list_head.MapWithLock{}},
		// {"Map                ", 100, 100000, 0, &list_head.Map{}},
		// {"MapString          ", 100, 100000, 0, &list_head.MapString{}},
		// //{"RMap               ", 100, 100000, 0, list_head.NewRMap()},
		{"HMap               ", 100, 1000000, 0, newWrapHMap(list_head.NewHMap())},
		// {"RMap2              ", 100, 100000, 0, list_head.NewRMap2()},
		{"sync.Map           ", 100, 1000000, 0, syncMap{}},
		// // {"WithLock           ", 100, 100000, 10, &list_head.MapWithLock{}},
		// // {"Map                ", 100, 100000, 10, &list_head.Map{}},
		// // {"MapString          ", 100, 100000, 10, &list_head.MapString{}},
		// // {"RMap               ", 100, 100000, 10, list_head.NewRMap()},
		// // {"sync.Map           ", 100, 100000, 10, syncMap{}},

		// {"WithLock           ", 100, 100000, 50, &list_head.MapWithLock{}},
		// {"Map                ", 100, 100000, 50, &list_head.Map{}},
		// {"MapString          ", 100, 100000, 50, &list_head.MapString{}},
		// //{"RMap               ", 100, 100000, 50, list_head.NewRMap()},
		// {"HMap               ", 100, 100000, 50, list_head.NewHMap()},
		// {"RMap2              ", 100, 100000, 50, list_head.NewRMap2()},
		// {"sync.Map           ", 100, 100000, 50, syncMap{}},

		// {"shard Map          ", 100, 100000, 50, newShard(func(i int) list_head.MapGetSet { return &list_head.Map{} })},
		// {"shard sync.Map     ", 100, 100000, 50, newShard(func(i int) list_head.MapGetSet { return syncMap{} })},
		// {"shard MapString    ", 100, 100000, 50, newShard(func(i int) list_head.MapGetSet { return &list_head.MapString{} })},

		// {"WithLock           ", 100, 10000, 80, &list_head.MapWithLock{}},
		// {"Map                ", 100, 10000, 80, &list_head.Map{}},
		// {"MapString          ", 100, 10000, 80, &list_head.MapString{}},
		// {"sync.Map           ", 100, 10000, 80, syncMap{}},
		// {"RMap               ", 100, 10000, 80, &list_head.RMap{}},
		// {"shard Map          ", 100, 10000, 80, newShard(func(i int) list_head.MapGetSet { return &list_head.Map{} })},
		// {"shard sync.Map     ", 100, 10000, 80, newShard(func(i int) list_head.MapGetSet { return syncMap{} })},
		// {"shard MapString    ", 100, 10000, 80, newShard(func(i int) list_head.MapGetSet { return &list_head.MapString{} })},
	}

	for _, bm := range benchmarks {
		b.Run(fmt.Sprintf("%s %2d", bm.name, bm.percent), func(b *testing.B) {

			runBnech(b, bm.mapInf, bm.concurrent, bm.cnt, uint64(bm.percent))

		})
	}
}

func Test_RMap(t *testing.T) {

	m := list_head.NewRMap()

	m.Set("hoge", &list_head.ListHead{})
	v := &list_head.ListHead{}
	v.Init()
	m.Set("hoge1", v)
	m.Set("hoge3", v)
	m.Set("oge3", v)
	m.Set("3", v)
	m.ValidateDirty()
	m.Get("hoge1")

}

func Test_HmapEntry(t *testing.T) {

	tests := []struct {
		name   string
		wanted list_head.MapItem
		got    func(*list_head.ListHead) list_head.MapItem
	}{
		{
			name:   "SampleItem",
			wanted: &list_head.SampleItem{K: "hoge", V: "hoge value"},
			got: func(lhead *list_head.ListHead) list_head.MapItem {
				return (&list_head.EmptySampleHMapEntry).HmapEntryFromListHead(lhead).(list_head.MapItem)
			},
		},
		{
			name:   "entryHMap",
			wanted: list_head.NewEntryMap("hogeentry", "hogevalue"),
			got: func(lhead *list_head.ListHead) list_head.MapItem {
				return (list_head.EmptyEntryHMap).HmapEntryFromListHead(lhead).(list_head.MapItem)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wanted.Key(), tt.got(tt.wanted.PtrListHead()).Key())

		})
	}

	// s := &list_head.SampleItem{
	// 	K: "hoge",
	// 	V: "hoge value",
	// }

	// lhead := s.PtrListHead()
	// s2 := (&list_head.EmptySampleHMapEntry).HmapEntryFromListHead(lhead).(*list_head.SampleItem)

	// assert.Equal(t, s.K, s2.K)
	// assert.Equal(t, s.V, s2.V)

}

func Test_HMap(t *testing.T) {

	m := newWrapHMap(list_head.NewHMap(list_head.MaxPefBucket(32), list_head.BucketMode(list_head.CombineSearch2)))
	//m := list_head.NewHMap(list_head.MaxPefBucket(32), list_head.BucketMode(list_head.NestedSearchForBucket))
	//m := list_head.NewHMap(list_head.MaxPefBucket(32), list_head.BucketMode(list_head.LenearSearchForBucket))
	list_head.EnableStats = true
	levels := m.base.ActiveLevels()
	assert.Equal(t, 0, len(levels))
	a := list_head.MapHead{}
	_ = a
	m.Set("hoge", &list_head.ListHead{})

	v := &list_head.ListHead{}
	v.Init()
	m.Set("hoge1", v)
	m.Set("hoge3", v)
	m.Set("oge3", v)
	m.Set("3", v)
	//m.ValidateDirty()

	levels = m.base.ActiveLevels()
	assert.Equal(t, 2, len(levels))

	DumpHmap(m.base)

	for i := 0; i < 10000; i++ {
		m.Set(fmt.Sprintf("fuge%d", i), v)
	}

	_, success := m.Get("hoge1")

	assert.True(t, success)
	stat := list_head.DebugStats

	_, success = m.Get("1234")

	assert.False(t, success)
	stat = list_head.DebugStats
	_ = stat

	for i := 0; i < 10000; i++ {
		_, ok := m.Get(fmt.Sprintf("fuge%d", i))
		assert.Truef(t, ok, "not found key=%s", fmt.Sprintf("fuge%d", i))
		if !ok {
			list_head.BucketMode(list_head.NestedSearchForBucket)(m.base)
			_, ok = m.Get(fmt.Sprintf("fuge%d", i))
			list_head.BucketMode(list_head.CombineSearch)(m.base)
			_, ok = m.Get(fmt.Sprintf("fuge%d", i))
		}
	}
	DumpHmap(m.base)

	str := m.base.DumpEntry()
	//assert.Equal(t, 0, len(str))
	assert.NotEqual(t, 0, len(str))

}

func DumpHmap(h *list_head.HMap) {

	fmt.Printf("DumpBucketPerLevel\n")
	fmt.Printf("%s\n", h.DumpBucketPerLevel())
	fmt.Printf("!!!!!!!!!!!!DumpBucket\n")
	fmt.Printf("%s\n", h.DumpBucket())
	fmt.Printf("%s\n", h.DumpEntry())
	fmt.Printf("fail 0x%x\n", list_head.Failreverse)

}
