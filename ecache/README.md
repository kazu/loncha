ecache
==========
This provides the `ecache` which implements a fixed-size
thread safe LRU cache and low-heap allocation and high-performance by embedded header data to data.

Example
=======


```golang
import (
    "github.com/kazu/loncha/ecache"
    list_head "github.com/kazu/loncha/lista_encabezado"
)


type Record struct {
    Key string
    Value string
    list_head.ListHead
}

func (r *Record) CacheKey() string {
	return r.Key
}

func (r *Record) Offset() uintptr {
	return unsafe.Offsetof(r.ListHead)
}

func (r *Record) PtrListHead() *list_head.ListHead {
	return &(r.ListHead)
}

func (r *Record) FromListHead(head *list_head.ListHead) list_head.List {
	return (*Record)(list_head.ElementOf(r, head))
}


func main() {

    c := ecache.New(Max(100), LRU())

    c.FnKey = func(l *list_head.ListHead) string {
		result := (&Record{}).FromListHead(l).(*Record)
		if result == nil {
			return ""
		}
		return result.Key
	}

    for i := 0; i< 10 ; i++ {
        c.Set(&Result{
            Key: fmt.Sprintf("key%d", i),
            Value: fmt.Sprintf("val%d", i)})
    }


    r, e := c.Get("key2")

}


```