// Copyright 2019 Kazuhisa TAKEI<xtakei@rytr.jp>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lonacha

import (
	"math/rand"
	"reflect"

	"github.com/seehuhn/mt19937"
)

// Shuffle is shuffing slice order. if slice is not pointer of slice or not slice, return error
func Shuffle(slice interface{}, seeds ...int64) (e error) {

	rv, err := sliceElm2Reflect(slice)
	if err != nil {
		return err
	}

	length := rv.Len()
	if length == 0 {
		return
	}
	randMt := rand.New(mt19937.New())

	if len(seeds) > 0 {
		randMt.Seed(seeds[0])
	}

	swap := reflect.Swapper(rv.Interface())

	for i := 0; i < length; i++ {
		r := i + randMt.Intn(length-i)
		swap(r, i)
	}
	return
}
