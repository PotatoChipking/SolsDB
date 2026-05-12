package main

import (
	"fmt"
	"log"
	"os"

	"github.com/PotatoChipking/SolsDB/ethadapter"
	"github.com/PotatoChipking/SolsDB/ordereddb"
)

func main() {
	dir, err := os.MkdirTemp("", "solsdb-example-*")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ordinary := newMapKV()
	state, err := ordereddb.OpenFileStore(dir, ordereddb.Options{SegmentMaxBlocks: 2})
	if err != nil {
		log.Fatal(err)
	}
	db := ethadapter.New(ordinary, state, ethadapter.Config{})
	defer db.Close()

	root := fillHash(1)
	nodeHash := fillHash(2)
	if err := db.BeginBlock(1, root); err != nil {
		log.Fatal(err)
	}
	if err := db.Put([]byte("header:1"), []byte("ordinary chain data")); err != nil {
		log.Fatal(err)
	}
	if err := db.PutStateNode(1, nodeHash, []byte("rlp encoded trie node")); err != nil {
		log.Fatal(err)
	}
	if err := db.CommitBlock(1, root); err != nil {
		log.Fatal(err)
	}
	value, err := db.GetStateNode(1, nodeHash)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("state node: %s\n", value)
}

func fillHash(seed byte) ordereddb.NodeHash {
	var hash ordereddb.NodeHash
	for i := range hash {
		hash[i] = seed
	}
	return hash
}

type mapKV struct {
	data map[string][]byte
}

func newMapKV() *mapKV {
	return &mapKV{data: make(map[string][]byte)}
}

func (db *mapKV) Has(key []byte) (bool, error) {
	_, ok := db.data[string(key)]
	return ok, nil
}

func (db *mapKV) Get(key []byte) ([]byte, error) {
	value, ok := db.data[string(key)]
	if !ok {
		return nil, ordereddb.ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (db *mapKV) Put(key []byte, value []byte) error {
	db.data[string(key)] = append([]byte(nil), value...)
	return nil
}

func (db *mapKV) Delete(key []byte) error {
	delete(db.data, string(key))
	return nil
}

func (db *mapKV) NewBatch() ethadapter.Batch {
	return &mapBatch{db: db}
}

func (db *mapKV) NewIterator(prefix []byte, start []byte) ethadapter.Iterator {
	return emptyIterator{}
}

func (db *mapKV) Stat(property string) (string, error) {
	return "", nil
}

func (db *mapKV) Compact(start []byte, limit []byte) error {
	return nil
}

func (db *mapKV) Close() error {
	return nil
}

type mapBatch struct {
	db *mapKV
}

func (b *mapBatch) Put(key []byte, value []byte) error {
	return b.db.Put(key, value)
}

func (b *mapBatch) Delete(key []byte) error {
	return b.db.Delete(key)
}

func (b *mapBatch) ValueSize() int {
	return 0
}

func (b *mapBatch) Write() error {
	return nil
}

func (b *mapBatch) Reset() {}

type emptyIterator struct{}

func (emptyIterator) Next() bool    { return false }
func (emptyIterator) Error() error  { return nil }
func (emptyIterator) Key() []byte   { return nil }
func (emptyIterator) Value() []byte { return nil }
func (emptyIterator) Release()      {}
