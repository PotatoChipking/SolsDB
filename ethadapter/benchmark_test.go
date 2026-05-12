package ethadapter

import (
	"testing"

	"github.com/PotatoChipking/SolsDB/ordereddb"
)

func BenchmarkAdapterExplicitStateNodeRead(b *testing.B) {
	adapter := New(newMemoryKV(), ordereddb.NewMemStore(), Config{})
	root := testHash(71)
	hash := testHash(72)
	if err := adapter.BeginBlock(1, root); err != nil {
		b.Fatal(err)
	}
	if err := adapter.PutStateNode(1, hash, []byte("node")); err != nil {
		b.Fatal(err)
	}
	if err := adapter.CommitBlock(1, root); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := adapter.GetStateNode(1, hash); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAdapterOrdinaryKVRead(b *testing.B) {
	adapter := New(newMemoryKV(), ordereddb.NewMemStore(), Config{})
	if err := adapter.Put([]byte("header"), []byte("value")); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := adapter.Get([]byte("header")); err != nil {
			b.Fatal(err)
		}
	}
}
