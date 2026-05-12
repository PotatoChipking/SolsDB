package ordereddb

const (
	DefaultSegmentMaxBlocks = 4096
	DefaultSegmentMaxBytes  = 256 << 20
	DefaultHotWindowBlocks  = 256
	DefaultSegmentCacheSize = 64
)

type Options struct {
	SegmentMaxBlocks uint64
	SegmentMaxBytes  uint64
	HotWindowBlocks  uint64
	SegmentCacheSize int
}

func (o Options) WithDefaults() Options {
	if o.SegmentMaxBlocks == 0 {
		o.SegmentMaxBlocks = DefaultSegmentMaxBlocks
	}
	if o.SegmentMaxBytes == 0 {
		o.SegmentMaxBytes = DefaultSegmentMaxBytes
	}
	if o.HotWindowBlocks == 0 {
		o.HotWindowBlocks = DefaultHotWindowBlocks
	}
	if o.SegmentCacheSize <= 0 {
		o.SegmentCacheSize = DefaultSegmentCacheSize
	}
	return o
}

func (o Options) ShouldSeal(first, last BlockNumber, bytes uint64) bool {
	o = o.WithDefaults()
	if last < first {
		return false
	}
	blocks := uint64(last-first) + 1
	return blocks >= o.SegmentMaxBlocks || bytes >= o.SegmentMaxBytes
}
