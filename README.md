# loncha
A high-performance slice Utilities for Go.
* high-perfirmance slice filter/finder.
* generator for slice and linked-list 

## Installation

### slice Utility 

    go get github.com/kazu/loncha

### generator 

    go get github.com/kazu/loncha/cmd/gen

## QuickStart 

### slice Utility

slice utility dosent use reflect/interface operation.

```
    import "github.com/kazu/loncha"

    type GameObject struct {
        ID int
        Name string
        Pos []float
    }

    ...

    var objs []GameObject

    loncha.Find(&objs, func(i int) bool {
        return objs[i].ID == 6
    } 

    loncha.Filter(&objs, func(i int) bool {
        return objs[i].ID == 12
    } 

	loncha.Delete(&objs, func(i int) bool {
		return objs[i].ID == 555
	})

    // find one object with conditions.
    obj, err := Select(&objs, func(i int) bool {
		return slice[i].ID < 50
	})

    err = loncha.Shuffle(objs, 2)
```

## generate double-linked list of linux kernel list_head type

define base struct

```
package game_object

import (
    "github.com/kazu/loncha/list_head"


type Player struct {
    ID int
    Name string
    Hp int
    list_head.ListHead
}
```

generate linked-list

```
    gen game_object player.go Player container_list/container_list.gtpl > player_list.go
    cat player_list.go

    OR

    gen game_object player.go Player container_list/container_list.gtpl player_list.go
    cat player_list.go
```
## benchmark Result


### loncha.Uniq vs hand Uniq vs go-funk.Uniq
```
loncha.Uniq-16         	    			1000	    997543 ns/op	  548480 B/op	   16324 allocs/op
loncha.UniqWithSort-16 	    			1000	   2237924 ns/op	     256 B/op	       7 allocs/op
loncha.UniqWithSort(sort)-16         	1000	    260283 ns/op	     144 B/op	       4 allocs/op
hand_Uniq-16                          	1000	    427765 ns/op	  442642 B/op	       8 allocs/op
hand_Uniq_iface-16                    	1000	    808895 ns/op	  632225 B/op	    6322 allocs/op
go-funk.Uniq-16                       	1000	   1708396 ns/op	  655968 B/op	   10004 allocs/op
```

### loncha.Filter vs go-funk.Filter

```
loncha.Filter-16         	     100	     89142 ns/op	   82119 B/op	       4 allocs/op
loncha.Filter_pointer-16 	     100	       201 ns/op	       0 B/op	       0 allocs/op
hand_Filter_pointer-16   	     100	     24432 ns/op	   81921 B/op	       1 allocs/op
go-funk.Filter-16        	     100	   2370492 ns/op	  640135 B/op	   20004 allocs/op
go-funk.Filter_pointer-16        100	      1048 ns/op	      64 B/op	       2 allocs/op
```
