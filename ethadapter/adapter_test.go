package ethadapter

import (
	"testing"

	"github.com/PotatoChipking/SolsDB/ordereddb"
)

func TestAdapterRoutesStateNodesToSolsDB(t *testing.T) {
	kv := newMemoryKV()
	state := ordereddb.NewMemStore()
	adapter := New(kv, state, Config{})
	root := testHash(1)
	hash := testHash(2)
	key := ordereddb.NewNodeKey(10, hash).Bytes()
	if err := adapter.BeginBlock(10, root); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Put(key, []byte("node")); err != nil {
		t.Fatal(err)
	}
	if _, ok := kv.data[string(key)]; ok {
		t.Fatal("state node was written to ordinary kv backend")
	}
	if err := adapter.CommitBlock(10, root); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "node" {
		t.Fatalf("unexpected node value %q", value)
	}
}

func TestAdapterRoutesOrdinaryKeysToBackend(t *testing.T) {
	kv := newMemoryKV()
	adapter := New(kv, ordereddb.NewMemStore(), Config{})
	if err := adapter.Put([]byte("header"), []byte("value")); err != nil {
		t.Fatal(err)
	}
	if _, ok := kv.data["header"]; !ok {
		t.Fatal("ordinary key was not written to kv backend")
	}
	value, err := adapter.Get([]byte("header"))
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "value" {
		t.Fatalf("unexpected value %q", value)
	}
}

func TestAdapterExplicitStateNodeAPI(t *testing.T) {
	adapter := New(newMemoryKV(), ordereddb.NewMemStore(), Config{})
	root := testHash(7)
	hash := testHash(8)
	if err := adapter.BeginBlock(30, root); err != nil {
		t.Fatal(err)
	}
	if err := adapter.PutStateNode(30, hash, []byte("explicit")); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CommitBlock(30, root); err != nil {
		t.Fatal(err)
	}
	value, err := adapter.GetStateNode(30, hash)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "explicit" {
		t.Fatalf("unexpected value %q", value)
	}
}

func TestAdapterBatchRoutesMixedWrites(t *testing.T) {
	kv := newMemoryKV()
	state := ordereddb.NewMemStore()
	adapter := New(kv, state, Config{})
	root := testHash(3)
	hash := testHash(4)
	nodeKey := ordereddb.NewNodeKey(20, hash).Bytes()
	if err := adapter.BeginBlock(20, root); err != nil {
		t.Fatal(err)
	}
	batch := adapter.NewBatch()
	if err := batch.Put([]byte("receipt"), []byte("kv")); err != nil {
		t.Fatal(err)
	}
	if err := batch.Put(nodeKey, []byte("state")); err != nil {
		t.Fatal(err)
	}
	if err := batch.Write(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CommitBlock(20, root); err != nil {
		t.Fatal(err)
	}
	if _, ok := kv.data["receipt"]; !ok {
		t.Fatal("ordinary batch entry missing")
	}
	value, err := adapter.Get(nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "state" {
		t.Fatalf("unexpected state value %q", value)
	}
}

func TestAdapterIteratorStatCompactPassthrough(t *testing.T) {
	kv := newMemoryKV()
	adapter := New(kv, ordereddb.NewMemStore(), Config{})
	if err := adapter.Put([]byte("h1"), []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Put([]byte("h2"), []byte("two")); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Put([]byte("r1"), []byte("receipt")); err != nil {
		t.Fatal(err)
	}
	iter := adapter.NewIterator([]byte("h"), nil)
	defer iter.Release()
	var keys []string
	for iter.Next() {
		keys = append(keys, string(iter.Key()))
	}
	if err := iter.Error(); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != "h1" || keys[1] != "h2" {
		t.Fatalf("unexpected iterator keys %v", keys)
	}
	stat, err := adapter.Stat("entries")
	if err != nil {
		t.Fatal(err)
	}
	if stat != "3" {
		t.Fatalf("unexpected stat %q", stat)
	}
	if err := adapter.Compact(nil, nil); err != nil {
		t.Fatal(err)
	}
}

func testHash(seed byte) ordereddb.NodeHash {
	var hash ordereddb.NodeHash
	for i := range hash {
		hash[i] = seed
	}
	return hash
}
