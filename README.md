# loncha
A high-performance slice Utilities for Go.
* high-perfirmance slice filter/finder.
* linked-list generics template using [Gennry]

## Installation

### slice Utility 

    go get github.com/kazu/loncha

## QuickStart 

### slice Utility

slice utility dosent use reflect/interface operation.

```go
    import "github.com/kazu/loncha"

    type GameObject struct {
        ID int
        Name string
        Pos []float
    }

    ...

    var objs []GameObject

```

find object from slice

```go

    loncha.Find(&objs, func(i int) bool {
        return objs[i].ID == 6
    } 
```

filter/delete object via condition function

```go

    loncha.Filter(&objs, func(obj *GameObject) bool {
        return obj.ID == 12
    } 

	loncha.Delete(&objs, func(i int) bool {
		return objs[i].ID == 555
	})
```

select object with condition function

```go

    // find one object with conditions.
    obj, err := Select(&objs, func(i int) bool {
		return slice[i].ID < 50
	})
```

shuffle slice 

```go
    err = loncha.Shuffle(objs, 2)

    loncha.Reverse(objs)
```


got intersection from two slices


```go
    var obj2 []GameObject
    intersectedObj := InsertSect(obj, obj2)


    sort.Slice(objs, func(i int) bool {
        return objs[i].ID >=  objs[j].ID 
    })
    sort.Slice(objs2, func(i int) bool {
        return objs[i].ID >=  objs[j].ID 
    })

    intersect2 := IntersectSorted(obj, obj2, func(s []GameObject, i int) int {
        return s[i].ID
    })
```

subtraction from two slices
```go
    subtractObj := Sub(obj, obj2)

    subtract2 := SubSorted(obj, obj2, func(s []GameObject, i int) int {
        return s[i].ID
    })

```

Returns an object formed from operands via function


```go
	slice1 := []int{10, 6, 4, 2}

	sum := Inject(slice1, func(sum *int, t int) int {
		return *sum + t
	})
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

```console
    $ go get go get github.com/cheekybits/genny
    $ wget -q -O - "https://github.com/kazu/loncha/master/container_list/list.go" | genny  gen "ListEntry=Player" > player_list.go
    $ wget -q -O - "https://github.com/kazu/loncha/master/container_list/list_test.go" | genny  gen "ListEntry=Player" > player_list_test.go
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


## References 

- [Gennry]


[Gennry]: https://github.com/cheekybits/genny