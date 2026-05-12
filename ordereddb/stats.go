package ordereddb

type Stats struct {
	PutNodeCount       uint64
	GetNodeCount       uint64
	DuplicateNodeHits  uint64
	HotJournalBytes    uint64
	ManifestBytes      uint64
	SealedSegments     uint64
	SegmentCacheHits   uint64
	SegmentCacheMisses uint64
}
