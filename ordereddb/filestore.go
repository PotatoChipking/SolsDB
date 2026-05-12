package ordereddb

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const segmentFileExt = ".sdb"

type FileStore struct {
	mu       sync.RWMutex
	dir      string
	segDir   string
	opts     Options
	manifest *MemManifest
	closed   bool

	pending map[BlockNumber]map[NodeHash][]byte
	hot     map[BlockNumber]map[NodeHash][]byte
	roots   map[BlockNumber]NodeHash
	hotSize uint64

	nextSegmentID uint64
	segments      map[uint64]SegmentMeta
	segmentCache  map[uint64]*cachedSegment
	seenNodes     map[NodeHash]struct{}
	stats         Stats
}

type cachedSegment struct {
	file   *os.File
	reader *SegmentReader
}

func OpenFileStore(dir string, opts Options) (*FileStore, error) {
	opts = opts.WithDefaults()
	segDir := filepath.Join(dir, "segments")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		return nil, err
	}
	store := &FileStore{
		dir:          dir,
		segDir:       segDir,
		opts:         opts,
		manifest:     NewMemManifest(),
		pending:      make(map[BlockNumber]map[NodeHash][]byte),
		hot:          make(map[BlockNumber]map[NodeHash][]byte),
		roots:        make(map[BlockNumber]NodeHash),
		segments:     make(map[uint64]SegmentMeta),
		segmentCache: make(map[uint64]*cachedSegment),
		seenNodes:    make(map[NodeHash]struct{}),
	}
	if err := store.loadManifest(); err != nil {
		return nil, err
	}
	if err := store.cleanupTempSegments(); err != nil {
		return nil, err
	}
	if err := store.validateSegments(); err != nil {
		return nil, err
	}
	if err := store.loadHotJournal(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *FileStore) BeginBlock(block BlockNumber, root NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.pending[block] = make(map[NodeHash][]byte)
	s.roots[block] = root
	return nil
}

func (s *FileStore) PutNode(block BlockNumber, hash NodeHash, rlp []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	nodes := s.pending[block]
	if nodes == nil {
		nodes = make(map[NodeHash][]byte)
		s.pending[block] = nodes
	}
	nodes[hash] = cloneBytes(rlp)
	s.stats.PutNodeCount++
	if _, ok := s.seenNodes[hash]; ok {
		s.stats.DuplicateNodeHits++
	} else {
		s.seenNodes[hash] = struct{}{}
	}
	return nil
}

func (s *FileStore) DeleteNode(block BlockNumber, hash NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if nodes := s.pending[block]; nodes != nil {
		delete(nodes, hash)
	}
	if nodes := s.hot[block]; nodes != nil {
		delete(nodes, hash)
	}
	return nil
}

func (s *FileStore) CommitBlock(block BlockNumber, root NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	nodes, ok := s.pending[block]
	if !ok {
		return ErrNotFound
	}
	if err := s.appendHotJournalLocked(block, root, nodes); err != nil {
		return err
	}
	s.hot[block] = nodes
	s.roots[block] = root
	delete(s.pending, block)
	var size uint64
	for _, value := range nodes {
		size += uint64(NodeKeySize + 4 + len(value))
	}
	s.hotSize += size
	if err := s.manifest.PutBlock(BlockMeta{
		Number:    block,
		Root:      root,
		Status:    BlockCommitted,
		NodeCount: uint64(len(nodes)),
	}); err != nil {
		return err
	}
	return s.manifest.RecordStateRoot(block, root, false)
}

func (s *FileStore) AbortBlock(block BlockNumber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	delete(s.pending, block)
	return nil
}

func (s *FileStore) SealThrough(block BlockNumber) (SegmentMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return SegmentMeta{}, ErrClosed
	}
	return s.sealThroughLocked(block)
}

func (s *FileStore) sealThroughLocked(block BlockNumber) (SegmentMeta, error) {
	blocks := s.hotBlocksThrough(block)
	if len(blocks) == 0 {
		return SegmentMeta{}, ErrNotFound
	}
	first, last := blocks[0], blocks[len(blocks)-1]
	entries := s.segmentEntries(blocks)
	if len(entries) == 0 {
		return SegmentMeta{}, ErrNotFound
	}
	id := s.nextSegmentID
	s.nextSegmentID++
	name := segmentFileName(first, last)
	tmp := filepath.Join(s.segDir, fmt.Sprintf(".%s.tmp", name))
	final := filepath.Join(s.segDir, name)
	file, err := os.Create(tmp)
	if err != nil {
		return SegmentMeta{}, err
	}
	writer, err := NewSegmentWriter(file)
	if err != nil {
		_ = file.Close()
		return SegmentMeta{}, err
	}
	for _, entry := range entries {
		if err := writer.Append(entry); err != nil {
			_ = file.Close()
			return SegmentMeta{}, err
		}
	}
	meta, err := writer.Finish()
	if err == nil {
		err = file.Sync()
	}
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return SegmentMeta{}, err
	}
	if err := os.Rename(tmp, final); err != nil {
		return SegmentMeta{}, err
	}
	if err := syncDir(s.segDir); err != nil {
		return SegmentMeta{}, err
	}
	meta.ID = id
	meta.FileName = name
	meta.FirstBlock = first
	meta.LastBlock = last
	meta.Status = SegmentSealed
	if err := s.manifest.PutSegment(meta); err != nil {
		return SegmentMeta{}, err
	}
	for _, number := range blocks {
		if bm, err := s.manifest.GetBlock(number); err == nil {
			bm.SegmentID = id
			_ = s.manifest.PutBlock(bm)
		}
		delete(s.hot, number)
	}
	s.recomputeHotSize()
	s.segments[id] = meta
	s.stats.SealedSegments++
	if err := s.rewriteHotJournalLocked(); err != nil {
		return SegmentMeta{}, err
	}
	if err := s.persistManifestLocked(); err != nil {
		return SegmentMeta{}, err
	}
	return meta, nil
}

func (s *FileStore) GetNode(block BlockNumber, hash NodeHash) ([]byte, error) {
	s.recordGetNode()
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrClosed
	}
	if nodes := s.pending[block]; nodes != nil {
		if value, ok := nodes[hash]; ok {
			s.mu.RUnlock()
			return cloneBytes(value), nil
		}
	}
	if nodes := s.hot[block]; nodes != nil {
		if value, ok := nodes[hash]; ok {
			s.mu.RUnlock()
			return cloneBytes(value), nil
		}
	}
	segment, err := s.manifest.FindSegment(block)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	s.mu.RUnlock()
	return s.getSegmentNode(segment, block, hash)
}

func (s *FileStore) recordGetNode() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.GetNodeCount++
}

func (s *FileStore) Canonicalize(block BlockNumber, root NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if err := s.manifest.MarkCanonical(block, root); err != nil {
		return err
	}
	if err := s.manifest.RecordStateRoot(block, root, true); err != nil {
		return err
	}
	return s.persistManifestLocked()
}

func (s *FileStore) RecordStateRoot(block BlockNumber, root NodeHash, canonical bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if err := s.manifest.RecordStateRoot(block, root, canonical); err != nil {
		return err
	}
	return s.persistManifestLocked()
}

func (s *FileStore) ResolveRoot(root NodeHash) (RootMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return RootMeta{}, ErrClosed
	}
	return s.manifest.ResolveRoot(root)
}

func (s *FileStore) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

func (s *FileStore) RollbackTo(block BlockNumber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	for number := range s.pending {
		if number > block {
			delete(s.pending, number)
		}
	}
	for number := range s.hot {
		if number > block {
			delete(s.hot, number)
		}
	}
	s.recomputeHotSize()
	return s.rewriteHotJournalLocked()
}

func (s *FileStore) PruneBefore(block BlockNumber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	for id, meta := range s.segments {
		if meta.LastBlock < block {
			meta.Status = SegmentPrunable
			s.segments[id] = meta
			_ = s.manifest.MarkSegmentPrunable(id)
		}
	}
	return s.persistManifestLocked()
}

func (s *FileStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var err error
	for id, segment := range s.segmentCache {
		if cerr := segment.file.Close(); err == nil {
			err = cerr
		}
		delete(s.segmentCache, id)
	}
	return err
}

func (s *FileStore) hotBlocksThrough(block BlockNumber) []BlockNumber {
	blocks := make([]BlockNumber, 0, len(s.hot))
	for number := range s.hot {
		if number <= block {
			blocks = append(blocks, number)
		}
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i] < blocks[j] })
	if uint64(len(blocks)) > s.opts.SegmentMaxBlocks {
		blocks = blocks[:s.opts.SegmentMaxBlocks]
	}
	return blocks
}

func (s *FileStore) segmentEntries(blocks []BlockNumber) []SegmentEntry {
	var entries []SegmentEntry
	for _, block := range blocks {
		for hash, value := range s.hot[block] {
			entries = append(entries, SegmentEntry{
				Key:   NewNodeKey(block, hash),
				Value: cloneBytes(value),
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return string(entries[i].Key[:]) < string(entries[j].Key[:])
	})
	return entries
}

func (s *FileStore) recomputeHotSize() {
	var size uint64
	for block, nodes := range s.hot {
		_ = block
		for _, value := range nodes {
			size += uint64(NodeKeySize + 4 + len(value))
		}
	}
	s.hotSize = size
}

func segmentFileName(first, last BlockNumber) string {
	return fmt.Sprintf("state-%020d-%020d%s", first, last, segmentFileExt)
}

func (s *FileStore) getSegmentNode(meta SegmentMeta, block BlockNumber, hash NodeHash) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	reader, err := s.segmentReaderLocked(meta)
	if err != nil {
		return nil, err
	}
	return reader.GetNode(block, hash)
}

func (s *FileStore) segmentReaderLocked(meta SegmentMeta) (*SegmentReader, error) {
	if cached := s.segmentCache[meta.ID]; cached != nil {
		s.stats.SegmentCacheHits++
		return cached.reader, nil
	}
	s.stats.SegmentCacheMisses++
	path := filepath.Join(s.segDir, meta.FileName)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	reader, err := NewSegmentReader(file, info.Size())
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	s.evictSegmentIfNeededLocked()
	s.segmentCache[meta.ID] = &cachedSegment{file: file, reader: reader}
	return reader, nil
}

func (s *FileStore) evictSegmentIfNeededLocked() {
	if s.opts.SegmentCacheSize <= 0 || len(s.segmentCache) < s.opts.SegmentCacheSize {
		return
	}
	for id, segment := range s.segmentCache {
		_ = segment.file.Close()
		delete(s.segmentCache, id)
		return
	}
}
