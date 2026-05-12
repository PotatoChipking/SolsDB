package ethadapter

import (
	"errors"
	"sync"

	"github.com/PotatoChipking/SolsDB/ordereddb"
)

type Adapter struct {
	mu         sync.RWMutex
	kv         KeyValueStore
	state      ordereddb.Store
	lifecycle  ordereddb.ChainLifecycle
	classifier NodeKeyClassifier
	closed     bool
}

type Config struct {
	Classifier NodeKeyClassifier
}

func New(kv KeyValueStore, state ordereddb.Store, cfg Config) *Adapter {
	classifier := cfg.Classifier
	if classifier == nil {
		classifier = OrderedNodeKeyClassifier{}
	}
	var lifecycle ordereddb.ChainLifecycle
	if v, ok := state.(ordereddb.ChainLifecycle); ok {
		lifecycle = v
	}
	return &Adapter{
		kv:         kv,
		state:      state,
		lifecycle:  lifecycle,
		classifier: classifier,
	}
}

func (a *Adapter) Has(key []byte) (bool, error) {
	if _, err := a.Get(key); err != nil {
		if errors.Is(err, ordereddb.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (a *Adapter) Get(key []byte) ([]byte, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ordereddb.ErrClosed
	}
	if block, hash, ok := a.classifier.Classify(key); ok {
		a.mu.RUnlock()
		return a.state.GetNode(block, hash)
	}
	kv := a.kv
	a.mu.RUnlock()
	return kv.Get(key)
}

func (a *Adapter) Put(key []byte, value []byte) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ordereddb.ErrClosed
	}
	if block, hash, ok := a.classifier.Classify(key); ok {
		a.mu.RUnlock()
		return a.state.PutNode(block, hash, value)
	}
	kv := a.kv
	a.mu.RUnlock()
	return kv.Put(key, value)
}

func (a *Adapter) PutStateNode(block ordereddb.BlockNumber, hash ordereddb.NodeHash, rlp []byte) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ordereddb.ErrClosed
	}
	state := a.state
	a.mu.RUnlock()
	return state.PutNode(block, hash, rlp)
}

func (a *Adapter) GetStateNode(block ordereddb.BlockNumber, hash ordereddb.NodeHash) ([]byte, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return nil, ordereddb.ErrClosed
	}
	state := a.state
	a.mu.RUnlock()
	return state.GetNode(block, hash)
}

func (a *Adapter) DeleteStateNode(block ordereddb.BlockNumber, hash ordereddb.NodeHash) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ordereddb.ErrClosed
	}
	state := a.state
	a.mu.RUnlock()
	return state.DeleteNode(block, hash)
}

func (a *Adapter) Delete(key []byte) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ordereddb.ErrClosed
	}
	if block, hash, ok := a.classifier.Classify(key); ok {
		a.mu.RUnlock()
		return a.state.DeleteNode(block, hash)
	}
	kv := a.kv
	a.mu.RUnlock()
	return kv.Delete(key)
}

func (a *Adapter) NewBatch() Batch {
	return &adapterBatch{adapter: a}
}

func (a *Adapter) NewIterator(prefix []byte, start []byte) Iterator {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return errIterator{err: ordereddb.ErrClosed}
	}
	kv := a.kv
	a.mu.RUnlock()
	return kv.NewIterator(prefix, start)
}

func (a *Adapter) Stat(property string) (string, error) {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return "", ordereddb.ErrClosed
	}
	kv := a.kv
	a.mu.RUnlock()
	return kv.Stat(property)
}

func (a *Adapter) Compact(start []byte, limit []byte) error {
	a.mu.RLock()
	if a.closed {
		a.mu.RUnlock()
		return ordereddb.ErrClosed
	}
	kv := a.kv
	a.mu.RUnlock()
	return kv.Compact(start, limit)
}

func (a *Adapter) BeginBlock(block ordereddb.BlockNumber, root ordereddb.NodeHash) error {
	return a.state.BeginBlock(block, root)
}

func (a *Adapter) CommitBlock(block ordereddb.BlockNumber, root ordereddb.NodeHash) error {
	return a.state.CommitBlock(block, root)
}

func (a *Adapter) AbortBlock(block ordereddb.BlockNumber) error {
	return a.state.AbortBlock(block)
}

func (a *Adapter) Canonicalize(block ordereddb.BlockNumber, root ordereddb.NodeHash) error {
	if a.lifecycle == nil {
		return nil
	}
	return a.lifecycle.Canonicalize(block, root)
}

func (a *Adapter) RollbackTo(block ordereddb.BlockNumber) error {
	if a.lifecycle == nil {
		return nil
	}
	return a.lifecycle.RollbackTo(block)
}

func (a *Adapter) PruneBefore(block ordereddb.BlockNumber) error {
	if a.lifecycle == nil {
		return nil
	}
	return a.lifecycle.PruneBefore(block)
}

func (a *Adapter) Close() error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return nil
	}
	a.closed = true
	kv := a.kv
	state := a.state
	a.mu.Unlock()
	if err := kv.Close(); err != nil {
		_ = state.Close()
		return err
	}
	return state.Close()
}
