package ethadapter

import "github.com/PotatoChipking/SolsDB/ordereddb"

type NodeKeyClassifier interface {
	Classify(key []byte) (ordereddb.BlockNumber, ordereddb.NodeHash, bool)
}

type OrderedNodeKeyClassifier struct{}

func (OrderedNodeKeyClassifier) Classify(key []byte) (ordereddb.BlockNumber, ordereddb.NodeHash, bool) {
	block, hash, err := ordereddb.ParseNodeKey(key)
	return block, hash, err == nil
}
