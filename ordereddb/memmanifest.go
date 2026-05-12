package ordereddb

import "sync"

type MemManifest struct {
	mu       sync.RWMutex
	blocks   map[BlockNumber]BlockMeta
	segments map[uint64]SegmentMeta
	roots    map[NodeHash]RootMeta
}

func NewMemManifest() *MemManifest {
	return &MemManifest{
		blocks:   make(map[BlockNumber]BlockMeta),
		segments: make(map[uint64]SegmentMeta),
		roots:    make(map[NodeHash]RootMeta),
	}
}

func (m *MemManifest) LastCommitted() (BlockMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest BlockMeta
	found := false
	for _, meta := range m.blocks {
		if meta.Status == BlockCommitted || meta.Status == BlockCanonical {
			if !found || meta.Number > latest.Number {
				latest = meta
				found = true
			}
		}
	}
	if !found {
		return BlockMeta{}, ErrNotFound
	}
	return latest, nil
}

func (m *MemManifest) GetBlock(block BlockNumber) (BlockMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meta, ok := m.blocks[block]
	if !ok {
		return BlockMeta{}, ErrNotFound
	}
	return meta, nil
}

func (m *MemManifest) PutBlock(meta BlockMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blocks[meta.Number] = meta
	return nil
}

func (m *MemManifest) FindSegment(block BlockNumber) (SegmentMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, meta := range m.segments {
		if meta.Status != SegmentDeleted && meta.Contains(block) {
			return meta, nil
		}
	}
	return SegmentMeta{}, ErrNotFound
}

func (m *MemManifest) PutSegment(meta SegmentMeta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.segments[meta.ID] = meta
	return nil
}

func (m *MemManifest) MarkSegmentPrunable(id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, ok := m.segments[id]
	if !ok {
		return ErrNotFound
	}
	meta.Status = SegmentPrunable
	m.segments[id] = meta
	return nil
}

func (m *MemManifest) MarkSegmentDeleted(id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, ok := m.segments[id]
	if !ok {
		return ErrNotFound
	}
	meta.Status = SegmentDeleted
	m.segments[id] = meta
	return nil
}

func (m *MemManifest) MarkCanonical(block BlockNumber, root NodeHash) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, ok := m.blocks[block]
	if !ok {
		return ErrNotFound
	}
	meta.Root = root
	meta.Status = BlockCanonical
	m.blocks[block] = meta
	m.roots[root] = RootMeta{Root: root, Block: block, Canonical: true}
	return nil
}

func (m *MemManifest) RecordStateRoot(block BlockNumber, root NodeHash, canonical bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roots[root] = RootMeta{Root: root, Block: block, Canonical: canonical}
	return nil
}

func (m *MemManifest) ResolveRoot(root NodeHash) (RootMeta, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	meta, ok := m.roots[root]
	if !ok {
		return RootMeta{}, ErrNotFound
	}
	return meta, nil
}

func (m *MemManifest) MarkPruned(block BlockNumber) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	meta, ok := m.blocks[block]
	if !ok {
		return ErrNotFound
	}
	meta.Status = BlockPruned
	m.blocks[block] = meta
	return nil
}

func (m *MemManifest) Snapshot() ([]BlockMeta, []SegmentMeta, []RootMeta) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	blocks := make([]BlockMeta, 0, len(m.blocks))
	for _, meta := range m.blocks {
		blocks = append(blocks, meta)
	}
	segments := make([]SegmentMeta, 0, len(m.segments))
	for _, meta := range m.segments {
		segments = append(segments, meta)
	}
	roots := make([]RootMeta, 0, len(m.roots))
	for _, meta := range m.roots {
		roots = append(roots, meta)
	}
	return blocks, segments, roots
}
