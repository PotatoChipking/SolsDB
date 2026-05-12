package ethadapter

type errIterator struct {
	err error
}

func (i errIterator) Next() bool {
	return false
}

func (i errIterator) Error() error {
	return i.err
}

func (i errIterator) Key() []byte {
	return nil
}

func (i errIterator) Value() []byte {
	return nil
}

func (i errIterator) Release() {}
