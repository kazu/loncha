package list_head_test

import (
	"testing"

	list_head "github.com/kazu/loncha/lista_encabezado"
)

func BenchmarkListReuse(b *testing.B) {
	old := list_head.MODE_CONCURRENT
	list_head.MODE_CONCURRENT = true
	defer func() { list_head.MODE_CONCURRENT = old }()
	head, node := &list_head.ListHead{}, &list_head.ListHead{}
	head.InitAsEmpty()
	node.Init()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := head.Append(node); err != nil {
			b.Fatal(err)
		}
		if got := head.Len(); got != 1 {
			b.Fatalf("Len after Append = %d", got)
		}
		if _, purged := node.Purge(); purged != node {
			b.Fatal("Purge did not return the node")
		}
		if got := head.Len(); got != 0 {
			b.Fatalf("Len after Purge = %d", got)
		}
	}
}
