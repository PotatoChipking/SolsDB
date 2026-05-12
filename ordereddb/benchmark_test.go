package ordereddb

import "testing"

func BenchmarkMemStoreHotRead(b *testing.B) {
	store := NewMemStore()
	root := testHash(61)
	hash := testHash(62)
	if err := store.BeginBlock(1, root); err != nil {
		b.Fatal(err)
	}
	if err := store.PutNode(1, hash, []byte("node")); err != nil {
		b.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.GetNode(1, hash); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileStoreHotRead(b *testing.B) {
	store, err := OpenFileStore(b.TempDir(), Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	root := testHash(63)
	hash := testHash(64)
	if err := store.BeginBlock(1, root); err != nil {
		b.Fatal(err)
	}
	if err := store.PutNode(1, hash, []byte("node")); err != nil {
		b.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.GetNode(1, hash); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFileStoreSealedSegmentRead(b *testing.B) {
	store, err := OpenFileStore(b.TempDir(), Options{SegmentMaxBlocks: 1})
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	root := testHash(65)
	hash := testHash(66)
	if err := store.BeginBlock(1, root); err != nil {
		b.Fatal(err)
	}
	if err := store.PutNode(1, hash, []byte("node")); err != nil {
		b.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		b.Fatal(err)
	}
	if _, err := store.SealThrough(1); err != nil {
		b.Fatal(err)
	}
	if _, err := store.GetNode(1, hash); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.GetNode(1, hash); err != nil {
			b.Fatal(err)
		}
	}
}
