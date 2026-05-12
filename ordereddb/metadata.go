package ordereddb

type BlockStatus uint8

const (
	BlockPending BlockStatus = iota + 1
	BlockCommitted
	BlockCanonical
	BlockPruned
)

type SegmentStatus uint8

const (
	SegmentActive SegmentStatus = iota + 1
	SegmentSealed
	SegmentPrunable
	SegmentDeleted
)

type BlockMeta struct {
	Number    BlockNumber
	Root      NodeHash
	Status    BlockStatus
	SegmentID uint64
	NodeCount uint64
}

type SegmentMeta struct {
	ID         uint64
	FirstBlock BlockNumber
	LastBlock  BlockNumber
	FileName   string
	Size       uint64
	Checksum   uint32
	Status     SegmentStatus
}

type RootMeta struct {
	Root      NodeHash
	Block     BlockNumber
	Canonical bool
}

func (s SegmentMeta) Contains(block BlockNumber) bool {
	return s.FirstBlock <= block && block <= s.LastBlock
}

type Manifest interface {
	LastCommitted() (BlockMeta, error)
	GetBlock(block BlockNumber) (BlockMeta, error)
	PutBlock(meta BlockMeta) error
	FindSegment(block BlockNumber) (SegmentMeta, error)
	PutSegment(meta SegmentMeta) error
	MarkSegmentPrunable(id uint64) error
	MarkSegmentDeleted(id uint64) error
	RecordStateRoot(block BlockNumber, root NodeHash, canonical bool) error
	ResolveRoot(root NodeHash) (RootMeta, error)
	MarkCanonical(block BlockNumber, root NodeHash) error
	MarkPruned(block BlockNumber) error
}
