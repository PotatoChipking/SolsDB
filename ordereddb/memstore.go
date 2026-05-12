package ordereddb

import "sync"

type MemStore struct {
	mu     sync.RWMutex
	closed bool
	blocks map[BlockNumber]*memBlock
	nodes  map[NodeKey][]byte
}

type memBlock struct {
	meta    BlockMeta
	pending map[NodeKey][]byte
}

func NewMemStore() *MemStore {
	return &MemStore{
		blocks: make(map[BlockNumber]*memBlock),
		nodes:  make(map[NodeKey][]byte),
	}
}

func (s *MemStore) BeginBlock(block BlockNumber, root NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	s.blocks[block] = &memBlock{
		meta: BlockMeta{
			Number: block,
			Root:   root,
			Status: BlockPending,
		},
		pending: make(map[NodeKey][]byte),
	}
	return nil
}

func (s *MemStore) PutNode(block BlockNumber, hash NodeHash, rlp []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	b, ok := s.blocks[block]
	if !ok || b.meta.Status != BlockPending {
		b = &memBlock{
			meta: BlockMeta{
				Number: block,
				Status: BlockPending,
			},
			pending: make(map[NodeKey][]byte),
		}
		s.blocks[block] = b
	}
	key := NewNodeKey(block, hash)
	b.pending[key] = cloneBytes(rlp)
	return nil
}

func (s *MemStore) DeleteNode(block BlockNumber, hash NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	delete(s.nodes, NewNodeKey(block, hash))
	return nil
}

func (s *MemStore) CommitBlock(block BlockNumber, root NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	b, ok := s.blocks[block]
	if !ok {
		return ErrNotFound
	}
	for key, value := range b.pending {
		s.nodes[key] = cloneBytes(value)
	}
	b.pending = nil
	b.meta.Root = root
	b.meta.Status = BlockCommitted
	b.meta.NodeCount = uint64(len(b.pending))
	return nil
}

func (s *MemStore) AbortBlock(block BlockNumber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	b, ok := s.blocks[block]
	if !ok {
		return nil
	}
	if b.meta.Status == BlockPending {
		delete(s.blocks, block)
	}
	return nil
}

func (s *MemStore) GetNode(block BlockNumber, hash NodeHash) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrClosed
	}
	key := NewNodeKey(block, hash)
	if b := s.blocks[block]; b != nil && b.meta.Status == BlockPending {
		if value, ok := b.pending[key]; ok {
			return cloneBytes(value), nil
		}
	}
	value, ok := s.nodes[key]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneBytes(value), nil
}

func (s *MemStore) Canonicalize(block BlockNumber, root NodeHash) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	b, ok := s.blocks[block]
	if !ok || b.meta.Status == BlockPending {
		return ErrNotFound
	}
	b.meta.Root = root
	b.meta.Status = BlockCanonical
	return nil
}

func (s *MemStore) RollbackTo(block BlockNumber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	for number := range s.blocks {
		if number > block {
			delete(s.blocks, number)
		}
	}
	for key := range s.nodes {
		if key.BlockNumber() > block {
			delete(s.nodes, key)
		}
	}
	return nil
}

func (s *MemStore) PruneBefore(block BlockNumber) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	for number, b := range s.blocks {
		if number < block {
			b.meta.Status = BlockPruned
		}
	}
	for key := range s.nodes {
		if key.BlockNumber() < block {
			delete(s.nodes, key)
		}
	}
	return nil
}

func (s *MemStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func cloneBytes(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
