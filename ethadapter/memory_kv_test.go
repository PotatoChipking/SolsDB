package ethadapter

import (
	"sort"
	"strconv"
	"strings"

	"github.com/PotatoChipking/SolsDB/ordereddb"
)

type memoryKV struct {
	data map[string][]byte
}

func newMemoryKV() *memoryKV {
	return &memoryKV{data: make(map[string][]byte)}
}

func (db *memoryKV) Has(key []byte) (bool, error) {
	_, ok := db.data[string(key)]
	return ok, nil
}

func (db *memoryKV) Get(key []byte) ([]byte, error) {
	value, ok := db.data[string(key)]
	if !ok {
		return nil, ordereddb.ErrNotFound
	}
	return clone(value), nil
}

func (db *memoryKV) Put(key []byte, value []byte) error {
	db.data[string(key)] = clone(value)
	return nil
}

func (db *memoryKV) Delete(key []byte) error {
	delete(db.data, string(key))
	return nil
}

func (db *memoryKV) NewBatch() Batch {
	return &memoryKVBatch{db: db}
}

func (db *memoryKV) NewIterator(prefix []byte, start []byte) Iterator {
	var keys []string
	for key := range db.data {
		if len(prefix) > 0 && !strings.HasPrefix(key, string(prefix)) {
			continue
		}
		if len(start) > 0 && key < string(start) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return &memoryIterator{db: db, keys: keys, pos: -1}
}

func (db *memoryKV) Stat(property string) (string, error) {
	if property == "entries" {
		return strconv.Itoa(len(db.data)), nil
	}
	return "", nil
}

func (db *memoryKV) Compact(start []byte, limit []byte) error {
	return nil
}

func (db *memoryKV) Close() error {
	return nil
}

type memoryIterator struct {
	db   *memoryKV
	keys []string
	pos  int
}

func (i *memoryIterator) Next() bool {
	i.pos++
	return i.pos < len(i.keys)
}

func (i *memoryIterator) Error() error {
	return nil
}

func (i *memoryIterator) Key() []byte {
	if i.pos < 0 || i.pos >= len(i.keys) {
		return nil
	}
	return []byte(i.keys[i.pos])
}

func (i *memoryIterator) Value() []byte {
	if i.pos < 0 || i.pos >= len(i.keys) {
		return nil
	}
	return clone(i.db.data[i.keys[i.pos]])
}

func (i *memoryIterator) Release() {}

type memoryKVBatch struct {
	db  *memoryKV
	ops []batchOp
}

func (b *memoryKVBatch) Put(key []byte, value []byte) error {
	b.ops = append(b.ops, batchOp{key: clone(key), value: clone(value)})
	return nil
}

func (b *memoryKVBatch) Delete(key []byte) error {
	b.ops = append(b.ops, batchOp{delete: true, key: clone(key)})
	return nil
}

func (b *memoryKVBatch) ValueSize() int {
	var size int
	for _, op := range b.ops {
		size += len(op.value)
	}
	return size
}

func (b *memoryKVBatch) Write() error {
	for _, op := range b.ops {
		if op.delete {
			_ = b.db.Delete(op.key)
		} else {
			_ = b.db.Put(op.key, op.value)
		}
	}
	return nil
}

func (b *memoryKVBatch) Reset() {
	b.ops = b.ops[:0]
}
