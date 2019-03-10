# lonacha
A high-performance slice Utilities for Go.
* high-perfirmance slice filter/finder.
* generator for slice and linked-list 

## Installation

### slice Utility 

    go get github.com/kazu/lonacha

### generator 

    go get github.com/kazu/lonacha/cmd/gen

## QuickStart 

### slice Utility

slice utility dosent use reflect/interface operation.

```
    import "github.com/kazu/lonacha"

    type GameObject struct {
        ID int
        Name string
        Pos []float
    }

    ...

    var objs []GameObject

    lonacha.Find(&objs, func(i int) bool {
        return objs[i].ID == 6
    } 

    lonacha.Filter(&objs, func(i int) bool {
        return objs[i].ID == 12
    } 

	lonacha.Delete(&objs, func(i int) bool {
		return objs[i].ID == 555
	})

    // find one object with conditions.
    obj, err := Select(&objs, func(i int) bool {
		return slice[i].ID < 50
	})

    err = lonacha.Shuffle(objs, 2)
```

## generate double-linked list of linux kernel list_head type

define base struct

```
package game_object

type Player struct {
    ID int
    Name string
    Hp int
}
```

generate linked-list

```
    gen player.go game_object  Player container_list/container_list.gtpl > player_list.go
    cat player_list.go
```
