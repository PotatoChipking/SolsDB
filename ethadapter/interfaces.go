package ethadapter

import "io"

type KeyValueReader interface {
	Has(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
}

type KeyValueWriter interface {
	Put(key []byte, value []byte) error
	Delete(key []byte) error
}

type Batch interface {
	KeyValueWriter
	ValueSize() int
	Write() error
	Reset()
}

type Iterator interface {
	Next() bool
	Error() error
	Key() []byte
	Value() []byte
	Release()
}

type Batcher interface {
	NewBatch() Batch
}

type Iteratee interface {
	NewIterator(prefix []byte, start []byte) Iterator
}

type Stater interface {
	Stat(property string) (string, error)
}

type Compacter interface {
	Compact(start []byte, limit []byte) error
}

type KeyValueStore interface {
	KeyValueReader
	KeyValueWriter
	Batcher
	Iteratee
	Stater
	Compacter
	io.Closer
}
