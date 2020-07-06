# container-list genny generics

generate by [Genny]

double linked-list lik a kernel list_head.

"container/list" package is very slow, occur heavy heap allocation. you cannot use in LRU list.


see detail [Improving 'container/list'].

## generate


```console
    $ wget -q -O - "https://github.com/kazu/loncha/master/container_list/container_list.go" | genny  gen "ListEntry=Player" > player_list.go
    $ wget -q -O - "https://github.com/kazu/loncha/master/container_list/container_list_test.go" | genny  gen "ListEntry=Player" > player_list_test.go
```


## References 
- [Gennry]
- [Improving 'container/list']

[Gennry]: https://github.com/cheekybits/genny
[Improving 'container/list']: https://idea.popcount.org/2014-02-28-improving-containerlist/