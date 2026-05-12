package ethadapter

type batchOp struct {
	delete bool
	key    []byte
	value  []byte
}

type adapterBatch struct {
	adapter *Adapter
	ops     []batchOp
	size    int
}

func (b *adapterBatch) Put(key []byte, value []byte) error {
	b.ops = append(b.ops, batchOp{
		key:   clone(key),
		value: clone(value),
	})
	b.size += len(value)
	return nil
}

func (b *adapterBatch) Delete(key []byte) error {
	b.ops = append(b.ops, batchOp{
		delete: true,
		key:    clone(key),
	})
	return nil
}

func (b *adapterBatch) ValueSize() int {
	return b.size
}

func (b *adapterBatch) Write() error {
	for _, op := range b.ops {
		var err error
		if op.delete {
			err = b.adapter.Delete(op.key)
		} else {
			err = b.adapter.Put(op.key, op.value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *adapterBatch) Reset() {
	b.ops = b.ops[:0]
	b.size = 0
}

func clone(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
