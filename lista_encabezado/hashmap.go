// Copyright 2019-2201 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package loncha/list_head is like a kernel's LIST_HEAD
// list_head is used by loncha/gen/containers_list
package list_head

import (
	"errors"
	"fmt"
	"math/bits"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"
)

//const cntOfHampBucket = 32

type SearchMode byte

const (
	LenearSearchForBucket SearchMode = 0
	ReversSearchForBucket            = 1
	NestedSearchForBucket            = 2
	CombineSearch                    = 3
	CombineSearch2                   = 4

	NoItemSearchForBucket = 9 // test mode
	FalsesSearchForBucket = 10
)

type HMap struct {
	//buckets [cntOfHampBucket]ListHead
	buckets      bucket
	lastBucket   *ListHead
	len          int64
	maxPerBucket int
	start        *ListHead
	last         *ListHead

	modeForBucket SearchMode
	mu            sync.Mutex
	levelCache    [16]atomic.Value
}

type LevelHead ListHead

type bucket struct {
	level     int
	reverse   uint64
	len       int64
	start     *ListHead
	LevelHead ListHead
	ListHead
}

func (e *bucket) Offset() uintptr {
	return unsafe.Offsetof(e.ListHead)
}

func (e *bucket) OffsetLevel() uintptr {
	return unsafe.Offsetof(e.LevelHead)
}

func (e *bucket) PtrListHead() *ListHead {
	return &e.ListHead
}

func (e *bucket) PtrLevelHead() *ListHead {
	return &e.LevelHead
}

func (e *bucket) FromListHead(head *ListHead) List {
	return entryHMapFromListHead(head)
}

func bucketFromListHead(head *ListHead) *bucket {
	return (*bucket)(ElementOf(emptyBucket, head))
}

func bucketFromLevelHead(head *ListHead) *bucket {
	if head == nil {
		return nil
	}
	return (*bucket)(unsafe.Pointer(uintptr(unsafe.Pointer(head)) - emptyBucket.OffsetLevel()))
}

type entryHMap struct {
	key      interface{}
	value    interface{}
	k        uint64
	reverse  uint64
	conflict uint64
	ListHead
}

var (
	emptyEntryHMap *entryHMap = &entryHMap{}
	emptyBucket    *bucket    = &bucket{}
)

func (e *entryHMap) Offset() uintptr {
	return unsafe.Offsetof(e.ListHead)
}

func (e *entryHMap) PtrListHead() *ListHead {
	return &e.ListHead
}

func (e *entryHMap) FromListHead(head *ListHead) List {
	return entryHMapFromListHead(head)
}

func entryHMapFromListHead(head *ListHead) *entryHMap {
	return (*entryHMap)(ElementOf(emptyEntryHMap, head))
}

type OptHMap func(*HMap)

func MaxPefBucket(max int) OptHMap {

	return func(h *HMap) {
		h.maxPerBucket = max
	}
}

func BucketMode(mode SearchMode) OptHMap {
	return func(h *HMap) {
		h.modeForBucket = mode
	}
}

func NewHMap(opts ...OptHMap) *HMap {
	MODE_CONCURRENT = true
	hmap := &HMap{len: 0, maxPerBucket: 32}
	hmap.buckets.InitAsEmpty()
	hmap.buckets = *(bucketFromListHead(hmap.buckets.Prev()))
	hmap.lastBucket = hmap.buckets.Next()
	list := &ListHead{}
	list.InitAsEmpty()
	hmap.start = list.Prev()
	hmap.last = list.Next()
	hmap.modeForBucket = NestedSearchForBucket

	for _, opt := range opts {
		opt(hmap)
	}
	hmap.initLevelCache()

	// hmap := newHMap(opts...)
	return hmap
}

func (h *HMap) set(key, value interface{}) bool {
	k, conflict := KeyToHash(key)
	return h._set(k, conflict, key, value)
}

func (h *HMap) initBeforeSet() {
	if !h.notHaveBuckets() {
		return
	}

	btable := &bucket{
		level: 16,
		len:   0,
	}
	btable.reverse = ^uint64(0)
	btable.Init()
	btable.LevelHead.Init()

	empty := &entryHMap{
		key:      nil,
		value:    nil,
		k:        bits.Reverse64(btable.reverse),
		reverse:  btable.reverse,
		conflict: 0,
	}
	empty.Init()
	h.add(h.start.Prev(WaitNoM()).Next(WaitNoM()), empty)
	h.buckets.Prev().Next().insertBefore(&btable.ListHead)
	btable.start = &empty.ListHead

	levelBucket := h.levelBucket(btable.level)
	levelBucket.LevelHead.prev.next.insertBefore(&btable.LevelHead)
	h.setLevel(btable.level, levelBucket)

	// er := h.checklevelAll()
	// _ = er

	btablefirst := btable

	btable = &bucket{
		level: 1,
		len:   0,
	}
	btable.reverse = 0
	btable.Init()
	btable.LevelHead.Init()

	empty = &entryHMap{
		key:      nil,
		value:    nil,
		k:        bits.Reverse64(btable.reverse),
		reverse:  btable.reverse,
		conflict: 0,
	}
	empty.Init()
	//h.add(h.start.Prev(WaitNoM()).Next(WaitNoM()), empty)
	//h.add(btablefirst.start, empty)
	btablefirst.start.insertBefore(&empty.ListHead)

	//h.buckets.Prev().Next().insertBefore(&btable.ListHead)
	btablefirst.Next().insertBefore(&btable.ListHead)
	btable.start = &empty.ListHead
	//bucketFromListHead(btablefirst.Next()).LevelHead.insertBefore(&btable.LevelHead)
	levelBucket = h.levelBucket(btable.level)
	levelBucket.LevelHead.prev.next.insertBefore(&btable.LevelHead)
	h.setLevel(btable.level, levelBucket)

	if EnableStats {
		fmt.Printf("%s\n", h.DumpBucket())
		fmt.Printf("%s\n", h.DumpEntry())
	}
}

func (h *HMap) _set(k, conflict uint64, key, value interface{}) bool {

	h.initBeforeSet()

	var btable *bucket
	var addOpt HMethodOpt
	// for mask := ^uint64(0); mask != 0x0; mask >>= 8 {
	// 	btable = h.searchBucket(k & mask)
	// 	if btable != nil {
	// 		break
	// 	}
	// }
	btable = h.searchBucket(k)
	if btable != nil && btable.start == nil {
		_ = ""

	}
	if btable == nil || btable.start == nil {
		btable = &bucket{}
		btable.start = h.start.Prev(WaitNoM()).Next(WaitNoM())
	} else {
		addOpt = WithBucket(btable)
	}

	entry, cnt := h.find(btable.start, func(ehead *entryHMap) bool {
		return bits.Reverse64(k) <= bits.Reverse64(ehead.k) && ehead.key != nil
	})

	if entry != nil && entry.k == k && entry.conflict == conflict {
		entry.value = value
		if btable.level > 0 && cnt > int(btable.len) {
			btable.len = int64(cnt)
		}
		return true
	}
	var pEntry *entryHMap
	var tStart *ListHead

	if entry != nil {
		pEntry = entryHMapFromListHead(entry.Prev(WaitNoM()))
		erk := bits.Reverse64(entry.k)
		prk := bits.Reverse64(pEntry.k)
		rk := bits.Reverse64(k)
		_, _, _ = erk, prk, rk

		if bits.Reverse64(pEntry.k) < bits.Reverse64(k) {
			tStart = &pEntry.ListHead
		} else {
			_ = ""
		}
	}

	if tStart == nil {
		tStart = btable.start
	}

	entry = &entryHMap{
		key:      key,
		value:    value,
		k:        k,
		reverse:  bits.Reverse64(k),
		conflict: conflict,
	}
	entry.Init()
	if addOpt == nil {
		h.add(tStart, entry)
	} else {
		h.add(tStart, entry, addOpt)
	}
	atomic.AddInt64(&h.len, 1)
	if btable.level > 0 {
		atomic.AddInt64(&btable.len, 1)
	}
	return true

}

func (h *HMap) get(key interface{}) (interface{}, bool) {
	e, success := h._get(KeyToHash(key))
	if e == nil {
		return e, success
	}
	return e.value, success
}

type CondOfFinder func(ehead *entryHMap) bool

func CondOfFind(reverse uint64, l sync.Locker) CondOfFinder {

	return func(ehead *entryHMap) bool {

		if EnableStats {
			l.Lock()
			DebugStats[CntSearchEntry]++
			l.Unlock()
		}
		return reverse <= bits.Reverse64(ehead.k)
	}

}

var Failreverse uint64 = 0

func (h *HMap) _get(k, conflict uint64) (*entryHMap, bool) {

	// if e := h.search(KeyToHash(key)); e != nil {
	// 	return e.value, true
	// }

	// return nil, false
	//bucket := h.searchBucket(k)
	if EnableStats {
		h.mu.Lock()
		DebugStats[CntOfGet]++
		h.mu.Unlock()
	}
	var ebucket *bucket
	var bucket *bucket
	switch h.modeForBucket {
	case FalsesSearchForBucket:
		bucket = h.searchBucket4(k)
		break
	case NoItemSearchForBucket:
		bucket = h.searchBucket4(k)
		return nil, true

	case NestedSearchForBucket:
		bucket = h.searchBucket4(k)

		break
	case CombineSearch, CombineSearch2:

		e := h.searchKey(k, true)
		if e == nil {
			if Failreverse == 0 {
				Failreverse = bits.Reverse64(k)
			}
			return nil, false
		}
		if e.k != k || e.conflict != conflict {
			return nil, false
		}
		return e, true

	case ReversSearchForBucket:
		bucket = h.searchBucket2(k)
		break
	default:
		bucket = h.searchBucket(k)
		break
	}

	//bucket := h.rsearchBucket(k)
	// _ = b2

	//return nil, false

	if bucket == nil {
		return nil, false
	}
	rk := bits.Reverse64(k)
	_ = rk
	var e *entryHMap
	useBsearch := false
	useReverse := false

	if !useBsearch {

		if bucket.prev.Empty() {
			useReverse = false
		} else if useReverse {
			ebucket = bucketFromListHead(bucket.prev)
		}
		if useReverse && nearUint64(bucket.reverse, ebucket.reverse, bits.Reverse64(k)) == ebucket.reverse {
			e, _ = h.reverse(ebucket.start, func(ehead *entryHMap) bool {
				return rk <= bits.Reverse64(ehead.k)
			})
		} else {

			if bucket.reverse > bits.Reverse64(k) {
				e, _ = h.reverse(bucket.start, func(ehead *entryHMap) bool {
					return rk <= bits.Reverse64(ehead.k)
				})

			} else {
				e, _ = h.find(bucket.start, CondOfFind(rk, &h.mu))
				// cnt := 0
				// for cur := bucket.start; !cur.Empty(); cur = cur.DirectNext() {
				// 	e = entryHMapFromListHead(cur)
				// 	if rk <= bits.Reverse64(e.k) {
				// 		break
				// 	}
				// 	cnt++
				// }

			}
		}
	}
	if h.modeForBucket == FalsesSearchForBucket {
		return nil, true
	}

	if useBsearch {
		e, _ = h.bsearch(bucket, func(ehead *entryHMap) bool {
			return rk <= bits.Reverse64(ehead.k)
		})
	}
	// _ = e2

	// if e != e2 {
	// 	_ = "???"
	// }

	if e == nil {
		return nil, false
	}
	if e.k != k || e.conflict != conflict {
		return nil, false
	}
	//return nil, false

	return e, true

}

// func (h *HMap) search(k, conflict uint64) *entryHMap {

// 	for cur := h.buckets[k%cntOfHampBucket].Prev(WaitNoM()); !cur.Empty(); cur = cur.Next(WaitNoM()) {
// 		e := entryHMapFromListHead(cur)
// 		if e.k == k && e.conflict == conflict {
// 			return e
// 		}
// 	}
// 	return nil
// }

// func (h *HMap) bucketEnd() (result *bucket) {

// 	for cur := h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM()); !cur.Empty(); cur = cur.Next(WaitNoM()) {
// 		result = bucketFromListHead(cur)
// 	}
// }
func (h *HMap) notHaveBuckets() bool {
	return h.lastBucket.Next(WaitNoM()).Prev(WaitNoM()).Empty()
}
func (h *HMap) searchBucket2(k uint64) (result *bucket) {

	if nearUint64(0, ^uint64(0), k) != 0 {
		return h.searchBucket(k)
	}
	return h.rsearchBucket(k)
}

func levelMask(level int) (mask uint64) {
	mask = 0
	for i := 0; i < level; i++ {
		mask = (mask << 4) | 0xf
	}
	return
}

func (h *HMap) searchBucket4(k uint64) (result *bucket) {

	level := 1
	levelbucket := bucketFromLevelHead(h.levelBucket(level).LevelHead.prev.next)

	var pCur, nCur *bucket

	// cur = bucketFromLevelHead(cur.LevelHead.next)
	for cur := levelbucket; !cur.Empty(); {
		if EnableStats {
			h.mu.Lock()
			DebugStats[CntSearchBucket]++
			h.mu.Unlock()
		}

		cReverseNoMask := bits.Reverse64(k)
		_ = cReverseNoMask
		cReverse := bits.Reverse64(k & toMask(cur.level))
		if cReverse == cur.reverse {
			level++
			if EnableStats {
				h.mu.Lock()
			}
			nCur = FindBucketWithLevel2(&cur.ListHead, bits.Reverse64(k), level)
			if EnableStats {
				h.mu.Unlock()
			}
			if nCur == nil {
				return cur
			}
			if bits.Reverse64(k&toMask(nCur.level)) != nCur.reverse {
				return cur
			}
			cur = nCur
			level = nCur.level
			continue
		}
		if !cur.LevelHead.prev.Empty() {
			pCur = bucketFromLevelHead(cur.LevelHead.prev)
			if pCur.reverse > cReverse && cReverse > cur.reverse {
				return pCur
			}
		}
		if !cur.LevelHead.next.Empty() {
			nCur = bucketFromLevelHead(cur.LevelHead.next)
			if cur.reverse > cReverse && cReverse > nCur.reverse {
				return cur
			}
		}

		if cReverse < cur.reverse {
			if cur.LevelHead.next.Empty() {
				_ = "???"
			}
			cur = bucketFromLevelHead(cur.LevelHead.next)
			continue
		}
		if cur.LevelHead.prev.Empty() {
			return cur
		}
		cur = bucketFromLevelHead(cur.LevelHead.prev)

	}
	return nil
}

func (h *HMap) searchBucket3(k uint64) (result *bucket) {

	level := 1
	var bcur *bucket
	for cur := h.buckets.ListHead.prev.next; !cur.Empty(); {
		bcur = bucketFromListHead(cur)
		blevel := FindBucketWithLevel(cur, bits.Reverse64(k), level)
		if blevel != nil {
			cur = &blevel.ListHead
			level++
			continue
		}
		for cur := cur; !cur.Empty(); cur = cur.DirectNext() {
			bcur = bucketFromListHead(cur)
			if bits.Reverse64(k) > bcur.reverse {
				return bcur
			}
		}
		return nil
	}
	return nil
}

func FindBucketWithLevel2(chead *ListHead, reverse uint64, level int) *bucket {

	cBucket := bucketFromListHead(chead)
	if cBucket == nil {
		return nil
	}

	// バケットはつねに reverse より小さい
	if cBucket.reverse > reverse {
		if cBucket.LevelHead.next.Empty() {
			return nil
		}
		if !cBucket.LevelHead.next.Empty() {
			cBucket = bucketFromLevelHead(cBucket.LevelHead.next)
		}
	}

	pBucket := bucketFromLevelHead(cBucket.LevelHead.prev)
	var mReverse uint64
	var nCur *bucket
	for cur := &cBucket.ListHead; !cur.Empty() && cur != &pBucket.ListHead; {
		if EnableStats {
			//h.mu.Lock()
			DebugStats[CntLevelBucket]++
			//h.mu.Unlock()
		}
		cBucket = bucketFromListHead(cur)
		mReverse = (bits.Reverse64(toMask(cBucket.level)) & reverse)
		if cBucket.reverse == 0 {
			return nil
		}
		if cBucket.level != level {

			if cBucket.reverse > reverse {
				_ = "invalid"
			}
			cur = cur.prev
			continue
		}
		if mReverse == cBucket.reverse {
			return cBucket
		}
		if cBucket.reverse > mReverse {
			if cBucket.LevelHead.next.Empty() {
				return nil
			}
			nCur = bucketFromLevelHead(cBucket.LevelHead.next)
			if nCur.reverse < mReverse {
				return nCur
			}
			//return bucketFromLevelHead(cBucket.LevelHead.next)
			return nil
		}

		if cBucket.LevelHead.prev.Empty() {
			return cBucket
		}

		cur = &bucketFromLevelHead(cBucket.LevelHead.prev).ListHead
	}
	return nil
}

func FindBucketWithLevel(chead *ListHead, reverse uint64, level int) *bucket {

	cur := bucketFromListHead(chead)
	if cur == nil {
		return nil
	}
	cnt := -1
	for cur != nil && !cur.Empty() {
		cnt++
		if cur.reverse == 0 {
			return nil
		}
		if cur.level != level {

			if (bits.Reverse64(toMask(cur.level)) & reverse) == cur.reverse {
				if !cur.prev.Empty() {
					cur = bucketFromListHead(cur.prev)
					continue
				}
			}
			if (bits.Reverse64(toMask(cur.level)) & reverse) > cur.reverse {
				return bucketFromLevelHead(cur.LevelHead.prev)
			}

			chead2 := cur.DirectNext()
			if chead2 == nil || chead2.Empty() {
				cur = nil
				return nil
			}

			nCur := bucketFromListHead(chead2)
			if (bits.Reverse64(toMask(nCur.level)) & reverse) == nCur.reverse {
				return nil
			}
			cur = nCur
			continue
		}
		cReverse := (bits.Reverse64(toMask(level)) & reverse)
		if cReverse == cur.reverse {
			return cur
		}

		if cReverse > cur.reverse {
			if !cur.LevelHead.prev.Empty() {
				cur = bucketFromLevelHead(cur.LevelHead.prev)
				if cReverse < cur.reverse {
					return cur
				}
				continue
			}
			return nil
			//return bucketFromLevelHead(cur.LevelHead.prev)
		}
		if cur.LevelHead.Empty() || cur.LevelHead.next.Empty() {
			return nil
		}
		pcur := bucketFromLevelHead(cur.LevelHead.prev)
		ncur := bucketFromLevelHead(cur.LevelHead.next)
		_, _ = pcur, ncur

		cur = ncur
		_ = cur
	}
	return nil
}

func (h *HMap) searchBucket(k uint64) (result *bucket) {
	cnt := 0

	for cur := h.buckets.ListHead.prev.next; !cur.Empty(); cur = cur.DirectNext() {
		//for cur := h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM()); !cur.Empty(); cur = cur.Next(WaitNoM()) {
		bcur := bucketFromListHead(cur)
		if bits.Reverse64(k) > bcur.reverse {
			return bcur
		}
		cnt++
	}
	return
}

func (h *HMap) rsearchBucket(k uint64) (result *bucket) {
	cnt := 0

	for cur := h.lastBucket.next.prev; !cur.Empty(); cur = cur.prev {
		//for cur := h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM()); !cur.Empty(); cur = cur.Next(WaitNoM()) {
		bcur := bucketFromListHead(cur)
		if bits.Reverse64(k) <= bcur.reverse {
			return bucketFromListHead(bcur.next)
		}
		cnt++
	}
	return
}

func (h *HMap) Set(k string, v *ListHead) bool {
	return h.set(k, v)
}

func (h *HMap) Get(k string) (v *ListHead, t bool) {
	vinf, t := h.get(k)

	v, _ = vinf.(*ListHead)
	return v, t
}

func (h *HMap) eachEntry(start *ListHead, fn func(*entryHMap)) {

	for cur := start; !cur.Empty(); cur = cur.Next(WaitNoM()) {
		e := entryHMapFromListHead(cur)
		if e.key == nil {
			continue
		}
		fn(e)
	}
	return
}

func (h *HMap) each(start *ListHead, fn func(key, value interface{})) {

	for cur := start; !cur.Empty(); cur = cur.Next(WaitNoM()) {
		e := entryHMapFromListHead(cur)
		fn(e.key, e.value)
	}
	return
}

func (h *HMap) find(start *ListHead, cond func(*entryHMap) bool, opts ...searchArg) (result *entryHMap, cnt int) {

	conf := sharedSearchOpt
	previous := conf.Options(opts...)
	defer func() {
		if previous != nil {
			conf.Options(previous)
		}
	}()

	cnt = 0
	var e *entryHMap
	if start.Empty() {
		return
	}

	for cur := start; cur != cur.next; cur = cur.DirectNext() {
		e = entryHMapFromListHead(cur)
		// erk := bits.Reverse64(e.k)
		// _ = erk
		if conf.ignoreBucketEntry && e.key == nil {
			continue
		}
		if cond(e) {
			result = e
			return
		}
		cnt++
	}
	return nil, cnt
}

func (h *HMap) reverse(start *ListHead, cond func(*entryHMap) bool) (result *entryHMap, cnt int) {

	cnt = 0

	for cur := start; !cur.Empty(); cur = cur.prev {
		e := entryHMapFromListHead(cur)
		// erk := bits.Reverse64(e.k)
		// _ = erk
		if !cond(e) {
			result = entryHMapFromListHead(cur.next)
			return
		}
		cnt++
	}
	return nil, cnt
}

func middleListHead(oBegin, oEnd *ListHead) (middle *ListHead) {
	// begin := oBegin
	// end := oEnd
	b := oBegin.Empty()
	e := oEnd.Empty()
	_, _ = b, e
	cnt := 0
	for begin, end := oBegin, oEnd; !begin.Empty() && !end.Empty(); begin, end = begin.next, end.prev {
		if begin == end {
			return begin
		}
		if begin.prev == end {
			return begin
		}
		cnt++
	}
	return
}

func bsearchListHead(oBegin, oEnd *ListHead, cond func(*ListHead) bool) *ListHead {
	begin := oBegin
	end := oEnd
	middle := middleListHead(begin, end)
	for {
		if middle == nil {
			return nil
		}

		if cond(begin) {
			return begin
		}
		if cond(middle) {
			end = middle
			middle = middleListHead(begin, end)
			if end == middle {
				return middle
			}
			continue
		}
		if !cond(end) {
			return end
		}
		if begin == middle {
			return end
		}
		begin = middle
		middle = middleListHead(begin, end)
	}

}

func absDiffUint64(x, y uint64) uint64 {
	if x < y {
		return y - x
	}
	return x - y
}

func nearUint64(a, b, dst uint64) uint64 {
	if absDiffUint64(a, dst) > absDiffUint64(b, dst) {
		return b
	}
	return a

}

func (h *HMap) bsearch(sbucket *bucket, cond func(*entryHMap) bool) (result *entryHMap, cnt int) {
	if sbucket.Empty() || sbucket.prev.Empty() {
		return nil, 0
	}

	ebucket := bucketFromListHead(sbucket.prev)
	if sbucket.start.prev.next.Empty() || ebucket.start.prev.Empty() {
		return nil, 0
	}

	rhead := bsearchListHead(sbucket.start.prev.next, ebucket.start.prev, func(cur *ListHead) bool {
		e := entryHMapFromListHead(cur)
		return cond(e)
	})
	if rhead == nil {
		return nil, 0
	}
	result = entryHMapFromListHead(rhead)
	return

	// cnt = 0

	// for cur := start; !cur.Empty(); cur = cur.DirectNext() {
	// 	e := entryHMapFromListHead(cur)
	// 	if cond(e) {
	// 		result = e
	// 		return
	// 	}
	// 	cnt++
	// }
	// return nil, cnt
}

func (h *HMap) MakeBucket(ocur *ListHead, back int) {

	cur := ocur
	//for i := 0; i < 2; i++ {
	cur = cur.Prev(WaitNoM())
	//}

	e := entryHMapFromListHead(cur)
	cBucket := h.searchBucket(e.k)
	if cBucket == nil {
		return
	}
	nextBucket := bucketFromListHead(cBucket.prev)
	newReverse := cBucket.reverse / 2
	if nextBucket.reverse == ^uint64(0) && cBucket.reverse == 0 {
		newReverse = bits.Reverse64(0x1)
	} else if nextBucket.reverse == ^uint64(0) {
		newReverse += ^uint64(0) / 2
		newReverse += 1
	} else {
		newReverse += nextBucket.reverse / 2
	}

	nBucket := &bucket{
		reverse: newReverse,
		level:   0,
		len:     0,
	}
	for cur := bits.Reverse64(nBucket.reverse); cur != 0; cur >>= 4 {
		nBucket.level++
	}
	if nBucket.reverse == 0 && nBucket.level > 1 {
		fmt.Printf("invalid")
	}

	nBucket.Init()
	nBucket.LevelHead.Init()

	for cur := cBucket.start.prev.next; !cur.Empty(); cur = cur.Next(WaitNoM()) {
		nBucket.len++
		e := entryHMapFromListHead(cur)
		if bits.Reverse64(e.k) > nBucket.reverse {
			nBucket.start = cur.prev
			break
		}
	}
	//cBucket.len -= nBucket.len
	atomic.AddInt64(&cBucket.len, -nBucket.len)
	h.addBucket(nBucket)

	nextLevel := h.findNextLevelBucket(nBucket.reverse, nBucket.level)

	if nBucket.LevelHead.next == &nBucket.LevelHead {
		_ = "broken"
	}

	if nextLevel != nil {

		nextLevelBucket := bucketFromLevelHead(nextLevel)
		if nextLevelBucket.reverse < nBucket.reverse {
			nextLevel.insertBefore(&nBucket.LevelHead)
		} else if nextLevelBucket.reverse != nBucket.reverse {
			//nBucket.LevelHead.insertBefore(nextLevel)

			nextnextBucket := bucketFromLevelHead(nextLevel.next)
			_ = nextnextBucket
			nextLevel.next.insertBefore(&nBucket.LevelHead)
		}

		//nextLevel.insertBefore(&nBucket.LevelHead)
		var nNext, nPrev *bucket
		if !nBucket.LevelHead.prev.Empty() {
			nPrev = bucketFromLevelHead(nBucket.LevelHead.prev)
		}
		if !nBucket.LevelHead.next.Empty() {
			nNext = bucketFromLevelHead(nBucket.LevelHead.next)
		}
		_, _ = nNext, nPrev

	} else {
		_ = "???"
	}
	if nBucket.LevelHead.next == &nBucket.LevelHead {
		_ = "broken"
	}

	// nextLeveByCache := bucketFromLevelHead(h.levelBucket(nBucket.level).LevelHead.prev.next)
	// _ = nextLeveByCache

	// if nextLeveByCache.LevelHead.prev.Empty() && nextLeveByCache.LevelHead.next.Empty() {
	// 	nextLeveByCache.LevelHead.next.insertBefore(&nBucket.LevelHead)
	// 	o := h.levelBucket(nBucket.level)
	// 	_ = o
	// 	h.setLevel(nBucket.level, nextLeveByCache)
	// }

	// er := h.checklevelAll()
	// _ = er

	// if h.levelBucket(nBucket.level) == nil {
	// 	h.setLevel(nBucket.level, nBucket)
	// }

	if int(nBucket.len) > h.maxPerBucket {
		h.MakeBucket(cBucket.start.next, int(nBucket.len)/2)
	}
	if int(cBucket.len) > h.maxPerBucket {
		h.MakeBucket(nextBucket.start.prev, int(nBucket.len)/2)
	}

	return

}

type hmapMethod struct {
	bucket *bucket
}

type HMethodOpt func(*hmapMethod)

func WithBucket(b *bucket) func(*hmapMethod) {

	return func(conf *hmapMethod) {
		conf.bucket = b
	}
}

func (h *HMap) add(start *ListHead, e *entryHMap, opts ...HMethodOpt) bool {
	var opt *hmapMethod
	if len(opts) > 0 {
		opt = &hmapMethod{}
		for _, fn := range opts {
			fn(opt)
		}
	}

	cnt := 0
	pos, _ := h.find(start, func(ehead *entryHMap) bool {
		cnt++
		return bits.Reverse64(e.k) < bits.Reverse64(ehead.k)
	}, ignoreBucketEntry(false))

	defer func() {
		if h.isEmptyBylevel(1) {
			return
		}

		if h.SearchKey(e.k, ignoreBucketEntry(false)) == nil {
			sharedSearchOpt.Lock()
			sharedSearchOpt.e = errors.New("add: item is added. but not found")
			sharedSearchOpt.Unlock()
			// fmt.Printf("%s\n", h.DumpBucket())
			// fmt.Printf("%s\n", h.DumpBucketPerLevel())
			// fmt.Printf("%s\n", h.DumpEntry())
			// h.searchKey(e.k, ignoreBucketEntry(false))
			// fmt.Printf("fail store")
		}
	}()

	if pos != nil {
		pos.insertBefore(&e.ListHead)
		if opt == nil || opt.bucket == nil {
			return true
		}
		btable := opt.bucket
		if btable != nil && e.key != nil && int(btable.len) > h.maxPerBucket {
			//if cnt > h.maxPerBucket && pos.key != nil {
			h.MakeBucket(&e.ListHead, int(btable.len)/2)
		}
		return true
	}
	if opt != nil && opt.bucket != nil {
		//opt.bucket.start.insertBefore(&e.ListHead)
		opt.bucket.entry().nextAsE().insertBefore(&e.ListHead)
		return true
	}
	h.last.insertBefore(&e.ListHead)

	return true
}

func (h *HMap) DumpBucket() string {
	var b strings.Builder

	for cur := h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM()); !cur.Empty(); cur = cur.Next(WaitNoM()) {
		btable := bucketFromListHead(cur)
		fmt.Fprintf(&b, "bucket{reverse: 0x%16x, len: %d, start: %p, level{%d, cur: %p, prev: %p next: %p}}\n",
			btable.reverse, btable.len, btable.start, btable.level, &btable.LevelHead, btable.LevelHead.prev, btable.LevelHead.next)
	}
	return b.String()
}

func (h *HMap) DumpBucketPerLevel() string {
	var b strings.Builder

	for i := range h.levelCache {
		cBucket := h.levelBucket(i + 1)
		if cBucket == nil {
			continue
		}
		if h.isEmptyBylevel(i + 1) {
			continue
		}
		fmt.Fprintf(&b, "bucket level=%d\n", i+1)

		for cur := cBucket.LevelHead.prev.next; !cur.Empty(); {
			cBucket = bucketFromLevelHead(cur)
			cur = cBucket.LevelHead.next
			fmt.Fprintf(&b, "bucket{reverse: 0x%16x, len: %d, start: %p, level{%d, cur: %p, prev: %p next: %p}}\n",
				cBucket.reverse, cBucket.len, cBucket.start, cBucket.level, &cBucket.LevelHead, cBucket.LevelHead.prev, cBucket.LevelHead.next)
		}
	}

	return b.String()
}

func (h *HMap) DumpEntry() string {
	var b strings.Builder

	for cur := h.start.Prev(WaitNoM()).Next(WaitNoM()); !cur.Empty(); cur = cur.Next(WaitNoM()) {
		e := entryHMapFromListHead(cur)
		fmt.Fprintf(&b, "entryHMap{key: %+10v, k: 0x%16x, reverse: 0x%16x), conflict: 0x%x, cur: %p, prev: %p, next: %p}\n",
			e.key, e.k, e.reverse, e.conflict, &e.ListHead, e.prev, e.next)
	}
	// a := b.String()
	// _ = a
	// fmt.Printf("!!!%s!!!!\n", b.String())
	return b.String()
}

func toMask(level int) (mask uint64) {

	for i := 0; i < level; i++ {
		if mask == 0 {
			mask = 0xf
			continue
		}
		mask = (mask << 4) | 0xf
	}
	return
}

func (h *HMap) _insertBefore(tBtable *ListHead, nBtable *bucket) {

	empty := &entryHMap{
		key:      nil,
		value:    nil,
		k:        bits.Reverse64(nBtable.reverse),
		reverse:  nBtable.reverse,
		conflict: 0,
	}
	empty.Init()
	var thead *ListHead
	if tBtable.Empty() {
		thead = h.start.Prev(WaitNoM()).Next(WaitNoM())
	} else {
		tBucket := bucketFromListHead(tBtable)
		thead = tBucket.start.Prev(WaitNoM()).Next(WaitNoM())
	}
	// h.start.Prev(WaitNoM()).Next(WaitNoM())
	h.add(thead, empty)
	tBtable.insertBefore(&nBtable.ListHead)
	nBtable.start = &empty.ListHead
}

func (h *HMap) addBucket(nBtable *bucket) {

	bstart := h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM())
RETRY:
	for bcur := h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM()); !bcur.Empty(); bcur = bcur.Next(WaitNoM()) {
		cBtable := bucketFromListHead(bcur)
		if cBtable.reverse == nBtable.reverse {
			return
		}

		if cBtable.reverse < nBtable.reverse {
			h._insertBefore(&cBtable.ListHead, nBtable)
			//cBtable.insertBefore(&nBtable.ListHead)
			if nBtable.reverse <= cBtable.reverse {
				_ = "???"
			}
			return
		}
	}

	bstart = h.buckets.ListHead.Prev(WaitNoM()).Next(WaitNoM())
	breverse := bucketFromListHead(bstart).reverse
	_ = breverse
	bbrev := bucketFromListHead(h.lastBucket.Next().Prev())
	_ = bbrev
	if nBtable.reverse <= bucketFromListHead(bstart).reverse {
		if bbrev.reverse > nBtable.reverse {
			//bbrev.Next().insertBefore(&nBtable.ListHead)
			h._insertBefore(bbrev.Next(), nBtable)
			return
		} else {
			_ = "???"
			goto RETRY
		}
	}
	//bstart.insertBefore(&nBtable.ListHead)
	h._insertBefore(bstart, nBtable)
}

func (h *HMap) findNextLevelBucket(reverse uint64, level int) (cur *ListHead) {

	// for cur := h.buckets.Prev().Next(); !cur.Empty(); cur = cur.DirectNext() {
	// 	bcur := bucketFromListHead(cur)
	// 	if bcur.level != level {
	// 		continue
	// 	}
	// 	if reverse > bcur.reverse {
	// 		return bcur
	// 	}

	// }
	// return nil
	bcur := h.levelBucket(level)
	if bcur == nil {
		return nil
	}
	front := bcur.LevelHead.Front()
	bcur = bucketFromLevelHead(front.prev.next)

	for cur := &bcur.LevelHead; true; cur = cur.next {
		if cur.Empty() {
			return cur
		}
		bcur := bucketFromLevelHead(cur)
		if reverse > bcur.reverse {
			return &bcur.LevelHead
		}
	}
	return nil
}

func (h *HMap) initLevelCache() {

	h.mu.Lock()
	defer h.mu.Unlock()

	for i := range h.levelCache {
		b := &bucket{level: i + 1}
		b.LevelHead.InitAsEmpty()
		//b.InitAsEmpty()
		h.levelCache[i].Store(b)
	}
}

func (h *HMap) setLevel(level int, b *bucket) bool {

	return false
	// if len(h.levelCache) <= level-1 {
	// 	return false
	// }

	// //ov := h.levelCache[level-1]
	// obucket := h.levelCache[level-1].Load().(*bucket)
	// success := h.levelCache[level-1].CompareAndSwap(obucket, b)
	// return success
}

func (h *HMap) levelBucket(level int) (b *bucket) {
	ov := h.levelCache[level-1]
	b = ov.Load().(*bucket)

	return b
}

func (h *HMap) isEmptyBylevel(level int) bool {
	if len(h.levelCache) < level {
		return true
	}
	b := h.levelBucket(level)

	if b.Empty() {
		return true
	}
	if b.LevelHead.prev.Empty() && b.LevelHead.next.Empty() {
		return true
	}
	return false
}

func (h *HMap) checklevelAll() error {

	for i := range h.levelCache {
		b := h.levelBucket(i + 1)
		if err := b.checklevel(); err != nil {
			return err
		}
	}
	return nil

}

func (b *bucket) checklevel() error {

	level := -1
	var reverse uint64
	for cur := b.LevelHead.next; !cur.Empty(); cur = cur.next {
		b := bucketFromLevelHead(cur)
		if level == -1 {
			level = bucketFromLevelHead(cur).level
			reverse = b.reverse
			continue
		}
		if level != bucketFromLevelHead(cur).level {
			return errors.New("invalid level")
		}
		if reverse < b.reverse {
			return errors.New("invalid reverse")
		}
		reverse = b.reverse
	}
	level = -1
	for cur := b.LevelHead.prev; !cur.Empty(); cur = cur.prev {
		b := bucketFromLevelHead(cur)
		if level == -1 {
			level = bucketFromLevelHead(cur).level
			reverse = b.reverse
			continue
		}
		if level != bucketFromLevelHead(cur).level {
			return errors.New("invalid level")
		}
		if reverse > b.reverse {
			return errors.New("invalid reverse")
		}
		reverse = b.reverse
	}
	return nil

}

type statKey byte

var EnableStats bool = false

const (
	CntSearchBucket  statKey = 1
	CntLevelBucket   statKey = 2
	CntSearchEntry   statKey = 3
	CntReverseSearch statKey = 4
	CntOfGet         statKey = 5
)

var DebugStats map[statKey]int = map[statKey]int{}

func (b *bucket) nextAsB() *bucket {

	if b.ListHead.next.Empty() {
		return b
	}

	return bucketFromListHead(b.ListHead.next)

}

func (b *bucket) prevAsB() *bucket {

	if b.ListHead.prev.Empty() {
		return b
	}

	return bucketFromListHead(b.ListHead.prev)

}

func (b *bucket) NextOnLevel() *bucket {

	if b.LevelHead.next.Empty() {
		return b
	}

	return bucketFromLevelHead(b.LevelHead.next)

}

func (b *bucket) PrevOnLevel() *bucket {

	if b.LevelHead.prev.Empty() {
		return b
	}

	return bucketFromLevelHead(b.LevelHead.prev)

}

func (b *bucket) NextEntry() *entryHMap {

	if b.start == nil {
		return nil
	}
	start := b.start
	if !start.next.Empty() {
		start = start.next
	}

	if !start.Empty() {
		return entryHMapFromListHead(start)
	}

	return nil

}

func (b *bucket) PrevEntry() *entryHMap {

	if b.start == nil {
		return nil
	}
	start := b.start
	if !start.prev.Empty() {
		start = start.prev
	}

	if !start.Empty() {
		return entryHMapFromListHead(start)
	}

	return nil

}

func (b *bucket) entry() *entryHMap {
	if b.start == nil {
		return nil
	}
	start := b.start
	if !start.Empty() {
		return entryHMapFromListHead(start)
	}
	return b.NextEntry()

}
func (e *entryHMap) nextNoCheck() *entryHMap {
	return entryHMapFromListHead(e.ListHead.next)
}

func (e *entryHMap) prevNoCheck() *entryHMap {
	return entryHMapFromListHead(e.ListHead.prev)
}

func (e *entryHMap) nextAsE() *entryHMap {
	start := &e.ListHead
	if !start.next.Empty() {
		start = start.next
	}
	if !start.Empty() {
		return entryHMapFromListHead(start)
	}
	return nil
}

func (e *entryHMap) prevAsE() *entryHMap {
	start := &e.ListHead
	if !start.prev.Empty() {
		start = start.prev
	}
	if !start.Empty() {
		return entryHMapFromListHead(start)
	}
	return nil
}

type searchOpt struct {
	h                 *HMap
	e                 error
	ignoreBucketEntry bool
	sync.Mutex
}

var sharedSearchOpt *searchOpt = &searchOpt{ignoreBucketEntry: true}

type searchArg func(*searchOpt) searchArg

func ignoreBucketEntry(t bool) searchArg {

	return func(opt *searchOpt) searchArg {
		prev := opt.ignoreBucketEntry
		opt.ignoreBucketEntry = t
		return ignoreBucketEntry(prev)
	}
}

func (o *searchOpt) Options(opts ...searchArg) (previous searchArg) {

	o.Lock()
	defer o.Unlock()
	for _, fn := range opts {
		previous = fn(o)
	}
	return previous
}
func (h *HMap) SearchKey(k uint64, opts ...searchArg) *entryHMap {

	conf := sharedSearchOpt
	previous := conf.Options(opts...)
	defer func() {
		if previous != nil {
			conf.Options(previous)
		}
	}()
	return h.searchKey(k, conf.ignoreBucketEntry)

}

func (h *HMap) searchKey(k uint64, ignoreBucketEnry bool) *entryHMap {

	conf := sharedSearchOpt

	level := 1
	topLevelBucket := h.levelBucket(level).PrevOnLevel().NextOnLevel()

	reverseNoMask := bits.Reverse64(k)
	var reverse uint64
	_ = reverse
	found := false
	var bCur *bucket

	levels := [16]*bucket{}

	var setLevelCache func(b *bucket)
	if h.modeForBucket == CombineSearch2 {

		levels[level-1] = topLevelBucket //uintptr(unsafe.Pointer(topLevelBucket))
		setLevelCache = func(b *bucket) {
			levels[b.level-1] = b //uintptr(unsafe.Pointer(b))
		}
	} else {
		setLevelCache = func(b *bucket) {}
	}

	for lbCur := topLevelBucket; true; lbCur = lbCur.NextOnLevel() {
	RETRY:
		setLevelCache(lbCur)
		if EnableStats && ignoreBucketEnry {
			h.mu.Lock()
			DebugStats[CntLevelBucket]++
			h.mu.Unlock()
		}
		reverse = bits.Reverse64(k & toMask(lbCur.level))

		if lbCur.reverse > reverseNoMask && lbCur != lbCur.NextOnLevel() {
			continue
		}
		var plbCur *bucket //:= lbCur.PrevOnLevel()
		var maxReverse uint64
		for plbCur = lbCur; plbCur.PrevOnLevel() != plbCur; plbCur = plbCur.PrevOnLevel() {
			if plbCur.reverse < reverseNoMask {
				continue
			}
			break
		}
		if plbCur == nil {
			plbCur = lbCur
		}
		if plbCur.reverse < reverseNoMask { // && plbCur.PrevOnLevel() == plbCur {

			if plbCur.level == 16 {
				goto EACH_ENTRY
			}
			if plbCur.level == 15 {
				goto EACH_ENTRY
			}
			if !h.isEmptyBylevel(plbCur.level + 1) {
				lbCur = h.levelBucket(plbCur.level + 1).PrevOnLevel().NextOnLevel()
				continue
			}

			goto EACH_ENTRY
		}

		if plbCur.reverse < reverseNoMask {
			conf.e = errors.New("searchKey: not invalid destinatioon")
			return nil
		}
		found = false
		maxReverse = (plbCur.reverse - reverseNoMask)
		if reverseNoMask-maxReverse > 0 {
			maxReverse = reverseNoMask - maxReverse
		} else {
			maxReverse = reverseNoMask
		}
		for bCur = plbCur; bCur.reverse > maxReverse; bCur = bCur.nextAsB() {
			if EnableStats && ignoreBucketEnry {
				h.mu.Lock()
				DebugStats[CntSearchBucket]++
				h.mu.Unlock()
			}
			if bCur.level == level+1 {
				level++
				found = true
				lbCur = bCur
				break
			}
		}
		setLevelCache(bCur)
		setLevelCache(lbCur)
		if !found {
			//lbCur = lbCur.prevAsB()
			if bCur.reverse > reverseNoMask {
				lbCur = bCur
			} else {
				lbCur = bCur.prevAsB()
			}
			setLevelCache(lbCur)
			goto EACH_ENTRY
		}

		if !h.isEmptyBylevel(level) {
			found = false
			goto RETRY
		}

	EACH_ENTRY:

		return h.searchBybucket(lbCur, levels, reverseNoMask, ignoreBucketEnry)
	}
	return nil
}

func (h *HMap) searchBybucket(lbCur *bucket, levels [16]*bucket, reverseNoMask uint64, ignoreBucketEnry bool) *entryHMap {

	lbNext := lbCur.NextOnLevel()
	if h.modeForBucket == CombineSearch2 {
		noNil := true
		for i, b := range levels {
			if b == nil {
				noNil = false
				break
			}
			if i+1 == lbNext.level {
				continue
			}
			//b := (*bucket)(unsafe.Pointer(p))
			if nearUint64(b.reverse, lbNext.reverse, reverseNoMask) == b.reverse {
				lbNext = b
			}
		}
		if noNil {
			noNil = false
		}
	}

	if lbNext.reverse < reverseNoMask {

		for cur := lbNext.entry(); cur != nil && !cur.Empty(); cur = cur.nextNoCheck() {
			if EnableStats && ignoreBucketEnry {
				h.mu.Lock()
				DebugStats[CntReverseSearch]++
				h.mu.Unlock()
			}
			if ignoreBucketEnry && cur.key == nil {
				continue
			}
			if cur.reverse < reverseNoMask {
				continue
			}
			if cur.reverse == reverseNoMask {
				return cur
			}
			return nil
		}
		return nil
	}

	for cur := lbNext.entry(); cur != nil && !cur.Empty(); cur = cur.prevNoCheck() {
		if EnableStats && ignoreBucketEnry {
			h.mu.Lock()
			DebugStats[CntSearchEntry]++
			h.mu.Unlock()
		}
		if ignoreBucketEnry && cur.key == nil {
			continue
		}
		if cur.reverse > reverseNoMask {
			continue
		}
		if cur.reverse == reverseNoMask {
			return cur
		}

		return nil

	}
	return nil

}

func (h *HMap) ActiveLevels() (result []int) {

	for i := range h.levelCache {
		if !h.isEmptyBylevel(i + 1) {
			result = append(result, i+1)
		}
	}
	return
}
