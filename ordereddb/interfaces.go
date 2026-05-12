package ordereddb

import "io"

type NodeReader interface {
	GetNode(block BlockNumber, hash NodeHash) ([]byte, error)
}

type NodeWriter interface {
	PutNode(block BlockNumber, hash NodeHash, rlp []byte) error
	DeleteNode(block BlockNumber, hash NodeHash) error
}

type BlockWriter interface {
	BeginBlock(block BlockNumber, root NodeHash) error
	PutNode(block BlockNumber, hash NodeHash, rlp []byte) error
	CommitBlock(block BlockNumber, root NodeHash) error
	AbortBlock(block BlockNumber) error
}

type Store interface {
	NodeReader
	NodeWriter
	BlockWriter
	io.Closer
}

type ChainLifecycle interface {
	Canonicalize(block BlockNumber, root NodeHash) error
	RollbackTo(block BlockNumber) error
	PruneBefore(block BlockNumber) error
}

type RootResolver interface {
	RecordStateRoot(block BlockNumber, root NodeHash, canonical bool) error
	ResolveRoot(root NodeHash) (RootMeta, error)
}

type Statser interface {
	Stats() Stats
}
