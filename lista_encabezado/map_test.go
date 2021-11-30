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
				_ = ok
				_, ok = m.Get(fmt.Sprintf("%d", index&mask))
				// if !ok {
				// 	_, ok = m.Get(fmt.Sprintf("%d", index&mask))
				// 	fmt.Printf("fail")
				// }
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
		{"RMap               ", 100, 100000, 0, list_head.NewRMap()},
		//{"RMap2              ", 100, 100000, 0, list_head.NewRMap2()},
		{"sync.Map           ", 100, 100000, 0, syncMap{}},
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
