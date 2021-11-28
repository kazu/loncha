package ecache_test

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/dgraph-io/ristretto"
	"github.com/stretchr/testify/assert"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/allegro/bigcache"
	"github.com/cespare/xxhash"
	"github.com/coocood/freecache"
	"github.com/golang/groupcache/lru"
	"github.com/kazu/loncha"
	"github.com/kazu/loncha/ecache"
	list_head "github.com/kazu/loncha/lista_encabezado"
	"github.com/pingcap/go-ycsb/pkg/generator"
)

// copy from https://github.com/dgraph-io/benchmarks/blob/master/cachebench/cache_bench_test.go
type Cache interface {
	Get(key []byte) ([]byte, error)
	Set(key []byte, value []byte) error
}

const (
	// based on 21million dataset, we observed a maximum key length of 77,
	// with minimum length being 6 and average length being 25. We also
	// observed that 99% of keys had length <64 bytes.
	maxKeyLength = 128
	// workloadSize is the size of array storing sequence of keys that we
	// have in our workload. In the benchmark, we iterate over this array b.N
	// number of times in circular fashion starting at a random position.
	workloadSize = 2 << 20
)

var (
	errKeyNotFound  = errors.New("key not found")
	errInvalidValue = errors.New("invalid value")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

//========================================================================
//                           Possible workloads
//========================================================================

func zipfKeyList() [][]byte {
	// To ensure repetition of keys in the array,
	// we are generating keys in the range from 0 to workloadSize/3.
	maxKey := int64(workloadSize) / 3

	// scrambled zipfian to ensure same keys are not together
	z := generator.NewScrambledZipfian(0, maxKey, generator.ZipfianConstant)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	keys := make([][]byte, workloadSize)
	for i := 0; i < workloadSize; i++ {
		keys[i] = []byte(strconv.Itoa(int(z.Next(r))))
	}

	return keys
}

func oneKeyList() [][]byte {
	v := rand.Int() % (workloadSize / 3)
	s := []byte(strconv.Itoa(v))

	keys := make([][]byte, workloadSize)
	for i := 0; i < workloadSize; i++ {
		keys[i] = s
	}

	return keys
}

//========================================================================
//                               ecache
//========================================================================

type ERecord struct {
	Key   string
	Value string
	ecache.CacheHead
}

func (r *ERecord) CacheKey() string {
	return r.Key
}

func (r *ERecord) Offset() uintptr {
	return unsafe.Offsetof(r.ListHead)
}

func (r *ERecord) FromListHead(head *list_head.ListHead) list_head.List {
	return (*ERecord)(list_head.ElementOf(&ERecord{}, head))
}

func (r *ERecord) PtrCacheHead() *ecache.CacheHead {
	return &(r.CacheHead)
}

type ECache struct {
	c *ecache.Cache
}

func (b *ECache) Get(key []byte) ([]byte, error) {

	// dd := b.c.GetHead(string(key))
	// if dd == nil {
	// 	return nil, ecache.ErrorNotFound
	// }

	// return []byte{}, nil

	d, e := b.c.Get(string(key))
	if e != nil {
		return nil, e
	}
	if d == nil {
		return nil, ecache.ErrorNotFound
	}
	result := d.(*ERecord)
	return []byte(result.Value), e
}

func (b *ECache) Set(key, value []byte) error {

	return b.c.SetFn(func(l *list_head.ListHead) ecache.CacheEntry {
		rec := b.c.DataFromListead(l)
		data, succ := rec.(*ERecord)
		if !succ {
			return data
		}
		data.Key = string(key)
		data.Value = string(value)
		return data
	})

}

func newECache(keysInWindow int) *ECache {

	cache := ecache.New(
		ecache.Max(keysInWindow*maxKeyLength),
		ecache.Sample(&ERecord{}),
		ecache.UseListPool(),
		ecache.AllocFn(func() ecache.CacheEntry { return new(ERecord) }),
		ecache.LRU())
	forValidate := 2 * workloadSize / 10
	for i := 0; i < 2*workloadSize; i++ {
		_ = cache.Set(&ERecord{Key: strconv.Itoa(i), Value: "data"})
		if i%forValidate == forValidate-1 {
			err := cache.ValidateTiny()
			_ = err
		}
	}
	err := cache.Validation()
	_ = err
	cache.Reset()

	return &ECache{cache}
}

//========================================================================
//                               BigCache
//========================================================================

type BigCache struct {
	c *bigcache.BigCache
}

func (b *BigCache) Get(key []byte) ([]byte, error) {
	return b.c.Get(string(key))
}

func (b *BigCache) Set(key, value []byte) error {
	return b.c.Set(string(key), value)
}

func newBigCache(keysInWindow int) *BigCache {
	cache, err := bigcache.NewBigCache(bigcache.Config{
		Shards:             256,
		LifeWindow:         0,
		MaxEntriesInWindow: keysInWindow,
		MaxEntrySize:       maxKeyLength,
		Verbose:            false,
	})
	if err != nil {
		panic(err)
	}

	// Enforce full initialization of internal structures. This is taken
	// from GetPutBenchmark.java from java caffeine. It is required in
	// caffeine given that it keeps buffering the keys for applying the
	// necessary changes later. This is probably unnecessary here.
	for i := 0; i < 2*workloadSize; i++ {
		_ = cache.Set(strconv.Itoa(i), []byte("data"))
	}
	_ = cache.Reset()

	return &BigCache{cache}
}

//========================================================================
//                               FastCache
//========================================================================

type FastCache struct {
	c *fastcache.Cache
}

func (b *FastCache) Get(key []byte) ([]byte, error) {
	v := b.c.Get(nil, key)
	if v == nil || len(v) == 0 {
		return nil, errKeyNotFound
	}

	return v, nil
}

func (b *FastCache) Set(key, value []byte) error {
	b.c.Set(key, value)
	return nil
}

func newFastCache(keysInWindow int) *FastCache {
	cache := fastcache.New(keysInWindow * maxKeyLength)

	// Enforce full initialization of internal structures. This is taken
	// from GetPutBenchmark.java from java caffeine. It is required in
	// caffeine given that it keeps buffering the keys for applying the
	// necessary changes later. This is probably unnecessary here.
	for i := 0; i < 2*workloadSize; i++ {
		cache.Set([]byte(strconv.Itoa(i)), []byte("data"))
	}
	cache.Reset()

	return &FastCache{cache}
}

//========================================================================
//                            FreeCache
//========================================================================

type FreeCache struct {
	c *freecache.Cache
}

func (f *FreeCache) Get(key []byte) ([]byte, error) {
	return f.c.Get(key)
}

func (f *FreeCache) Set(key, value []byte) error {
	return f.c.Set(key, value, 0)
}

func newFreeCache(keysInWindow int) *FreeCache {
	cache := freecache.NewCache(keysInWindow * maxKeyLength)

	// Enforce full initialization of internal structures
	// (probably not required, see above in bigcache)
	for i := 0; i < 2*workloadSize; i++ {
		_ = cache.Set([]byte(strconv.Itoa(i)), []byte("data"), 0)
	}
	cache.Clear()

	return &FreeCache{cache}
}

//========================================================================
//                            GroupCache
//========================================================================

const (
	segmentAndOpVal = 255
)

type GroupCache struct {
	shards [256]*lru.Cache
	locks  [256]sync.Mutex
}

func (g *GroupCache) Get(key []byte) ([]byte, error) {
	hashVal := xxhash.Sum64(key)
	shardNum := hashVal & segmentAndOpVal

	g.locks[shardNum].Lock()
	v, ok := g.shards[shardNum].Get(string(key))
	g.locks[shardNum].Unlock()

	if ok {
		return v.([]byte), nil
	}
	return nil, errKeyNotFound
}

func (g *GroupCache) Set(key, value []byte) error {
	hashVal := xxhash.Sum64(key)
	shardNum := hashVal & segmentAndOpVal

	g.locks[shardNum].Lock()
	g.shards[shardNum].Add(string(key), value)
	g.locks[shardNum].Unlock()

	return nil
}

func newGroupCache(keysInWindow int) *GroupCache {
	gc := &GroupCache{}
	for i := 0; i < 256; i++ {
		gc.shards[i] = lru.New(keysInWindow / 256)
	}

	// Enforce full initialization of internal structures
	for j := 0; j < 2*workloadSize; j++ {
		_ = gc.Set([]byte(strconv.Itoa(j)), []byte("data"))
	}
	for i := 0; i < 256; i++ {
		gc.shards[i].Clear()
	}

	return gc
}

//========================================================================
//                            Ristretto
//========================================================================

type RistrettoCache struct {
	c *ristretto.Cache
}

func (r *RistrettoCache) Get(key []byte) ([]byte, error) {
	v, ok := r.c.Get(key)
	if ok {
		return v.([]byte), nil
	} else {
		return nil, errKeyNotFound
	}
}

func (r *RistrettoCache) Set(key, value []byte) error {
	_ = r.c.Set(key, value, 0)
	return nil
}

func newRistretto(keysInWindow int) *RistrettoCache {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(keysInWindow * 10),
		MaxCost:     int64(keysInWindow),
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}

	return &RistrettoCache{cache}
}

//========================================================================
//                              sync.Map
//========================================================================

type SyncMap struct {
	c *sync.Map
}

func (m *SyncMap) Get(key []byte) ([]byte, error) {
	v, ok := m.c.Load(string(key))
	if !ok {
		return nil, errKeyNotFound
	}

	tv, ok := v.([]byte)
	if !ok {
		return nil, errInvalidValue
	}

	return tv, nil
}

func (m *SyncMap) Set(key, value []byte) error {
	// We are not performing any initialization here unlike other caches
	// given that there is no function available to reset the map.
	m.c.Store(string(key), value)
	return nil
}

func newSyncMap() *SyncMap {
	return &SyncMap{new(sync.Map)}
}

//========================================================================
//                         Benchmark Code
//========================================================================

func runCacheBenchmark(b *testing.B, cache Cache, keys [][]byte, pctWrites uint64) {
	b.ReportAllocs()

	size := len(keys)
	mask := size - 1
	rc := uint64(0)

	// initialize cache
	forValidate := size / 100
	_, isEcache := cache.(*ECache)
	for i := 0; i < size; i++ {
		_ = cache.Set(keys[i], []byte("data"))
		if isEcache && i%forValidate == 0 {
			err := cache.(*ECache).c.ValidateTiny()
			_ = err
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		index := rand.Int() & mask
		mc := atomic.AddUint64(&rc, 1)

		if pctWrites*mc/100 != pctWrites*(mc-1)/100 {
			for pb.Next() {
				_ = cache.Set(keys[index&mask], []byte("data"))
				index = index + 1
			}
		} else {
			for pb.Next() {
				_, _ = cache.Get(keys[index&mask])
				index = index + 1
			}
		}
	})
}

func BenchmarkCaches(b *testing.B) {
	zipfList := zipfKeyList()
	oneList := oneKeyList()
	_, _ = oneList, zipfList

	// two datasets (zipf, onekey)
	// 3 caches (bigcache, freecache, sync.Map)
	// 3 types of benchmark (read, write, mixed)
	benchmarks := []struct {
		name      string
		cache     Cache
		keys      [][]byte
		pctWrites uint64
	}{
		{"ECacheZipfRead", newECache(b.N), zipfList, 0},
		{"BigCacheZipfRead", newBigCache(b.N), zipfList, 0},
		{"FastCacheZipfRead", newFastCache(b.N), zipfList, 0},
		{"FreeCacheZipfRead", newFreeCache(b.N), zipfList, 0},
		{"GroupCacheZipfRead", newGroupCache(b.N), zipfList, 0},
		{"RistrettoZipfRead", newRistretto(b.N), zipfList, 0},
		{"SyncMapZipfRead", newSyncMap(), zipfList, 0},

		// {"ECacheOneKeyRead", newECache(b.N), oneList, 0},
		// {"BigCacheOneKeyRead", newBigCache(b.N), oneList, 0},
		// {"FastCacheOneKeyRead", newFastCache(b.N), oneList, 0},
		// {"FreeCacheOneKeyRead", newFreeCache(b.N), oneList, 0},
		// {"GroupCacheOneKeyRead", newGroupCache(b.N), oneList, 0},
		// {"RistrettoOneKeyRead", newRistretto(b.N), oneList, 0},
		// {"SyncMapOneKeyRead", newSyncMap(), oneList, 0},

		// {"ECacheZipfWrite", newECache(b.N), zipfList, 100},
		// {"BigCacheZipfWrite", newBigCache(b.N), zipfList, 100},
		// {"FastCacheZipfWrite", newFastCache(b.N), zipfList, 100},
		// {"FreeCacheZipfWrite", newFreeCache(b.N), zipfList, 100},
		// {"GroupCacheZipfWrite", newGroupCache(b.N), zipfList, 100},
		// {"RistrettoZipfWrite", newRistretto(b.N), zipfList, 100},
		// {"SyncMapZipfWrite", newSyncMap(), zipfList, 100},

		// {"ECacheOneKeyWrite", newECache(b.N), oneList, 100},
		// {"BigCacheOneKeyWrite", newBigCache(b.N), oneList, 100},
		// {"FastCacheOneKeyWrite", newFastCache(b.N), oneList, 100},
		// {"FreeCacheOneKeyWrite", newFreeCache(b.N), oneList, 100},
		// {"GroupCacheOneKeyWrite", newGroupCache(b.N), oneList, 100},
		// {"RistrettoOneKeyWrite", newRistretto(b.N), oneList, 100},
		// {"SyncMapOneKeyWrite", newSyncMap(), oneList, 100},

		// {"ECacheZipfMixed", newECache(b.N), zipfList, 25},
		// {"BigCacheZipfMixed", newBigCache(b.N), zipfList, 25},
		// {"FastCacheZipfMixed", newFastCache(b.N), zipfList, 25},
		// {"FreeCacheZipfMixed", newFreeCache(b.N), zipfList, 25},
		// {"GroupCacheZipfMixed", newGroupCache(b.N), zipfList, 25},
		// {"RistrettoZipfMixed", newRistretto(b.N), zipfList, 25},
		// {"SyncMapZipfMixed", newSyncMap(), zipfList, 25},

		// {"ECacheOneKeyMixed", newECache(b.N), oneList, 25},
		// {"BigCacheOneKeyMixed", newBigCache(b.N), oneList, 25},
		// {"FastCacheOneKeyMixed", newFastCache(b.N), oneList, 25},
		// {"FreeCacheOneKeyMixed", newFreeCache(b.N), oneList, 25},
		// {"GroupCacheOneKeyMixed", newGroupCache(b.N), oneList, 25},
		// {"RistrettoOneKeyMixed", newRistretto(b.N), oneList, 25},
		// {"SyncMapOneKeyMixed", newSyncMap(), oneList, 25},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			runCacheBenchmark(b, bm.cache, bm.keys, bm.pctWrites)
		})
	}
}

func Test_SimpleEcache(t *testing.T) {

	c := newECache(128)
	c.Set([]byte("1"), []byte("10"))
	c.Set([]byte("2"), []byte("20"))
	c.Set([]byte("3"), []byte("30"))
	time.Sleep(1 * time.Millisecond)
	d, e := c.Get([]byte("3"))
	_, _ = d, e

	assert.Equal(t, d, []byte("30"))
}

func Test_CacheEntry(t *testing.T) {

	var r ecache.CacheEntry
	r = &ERecord{Key: "a", Value: "b"}

	rhead := r.PtrListHead()

	empty := &ERecord{}

	rr := empty.FromListHead(rhead).(*ERecord)

	assert.Equal(t, "a", rr.Key)

}

type Result struct {
	QueryType byte
	Query     string
	Result    string
	ecache.CacheHead
}

func (r *Result) CacheKey() string {
	return r.Query
}

func (r *Result) Offset() uintptr {
	return unsafe.Offsetof(r.ListHead)
}

func (r *Result) FromListHead(head *list_head.ListHead) list_head.List {
	return (*Result)(list_head.ElementOf(&Result{}, head))
}

func (r *Result) PtrCacheHead() *ecache.CacheHead {
	return &(r.CacheHead)
}

func Test_SetFn(t *testing.T) {

	cache := ecache.New(
		ecache.Max(20),
		ecache.Sample(&Result{}),
		ecache.UseListPool(),
		ecache.AllocFn(func() ecache.CacheEntry { return new(Result) }),
		ecache.LRU())

	err := cache.SetFn(func(l *list_head.ListHead) ecache.CacheEntry {
		rec := cache.DataFromListead(l)
		data, succ := rec.(*Result)
		if !succ {
			return data
		}
		data.QueryType = 1
		data.Query = "query1"
		data.Result = "result1"
		return data
	})

	assert.NoError(t, err)

	err = cache.SetFn(func(l *list_head.ListHead) ecache.CacheEntry {
		rec := cache.DataFromListead(l)
		data, succ := rec.(*Result)
		if !succ {
			return data
		}
		data.QueryType = 1
		data.Query = "query2"
		data.Result = "result2"
		return data
	})

	assert.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	d, e := cache.Get("query1")
	assert.NoError(t, e)
	r, ok := d.(*Result)

	assert.NotNil(t, r)
	assert.True(t, ok)
	assert.Equal(t, "query1", r.CacheKey())
	assert.Equal(t, "result1", r.Result)

	cnt := 0
	keys := []string{"query1", "query2"}

	cache.ReverseEach(func(c ecache.CacheEntry) {
		cnt++
		found := loncha.Contain(&keys, func(i int) bool {
			return keys[i] == c.CacheKey()
		})
		assert.True(t, found)
	})
	assert.Equal(t, 2, cnt)
}

func Test_syncMap(t *testing.T) {

	a := sync.Map{}
	a.Store("aaa", 1)
	z, _ := a.Load("aaa")
	_ = z
	z, _ = a.Load("aaa")
	a.Store("aaa", 2)
	a.Store("aav", 3)
	z, _ = a.Load("aav")
	a.Delete("aaa")

}
