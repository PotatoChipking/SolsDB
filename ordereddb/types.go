package ordereddb

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	BlockNumberSize = 8
	NodeHashSize    = 32
	NodeKeySize     = BlockNumberSize + NodeHashSize
)

var (
	ErrInvalidNodeHash = errors.New("ordereddb: invalid node hash")
	ErrInvalidNodeKey  = errors.New("ordereddb: invalid node key")
	ErrNotFound        = errors.New("ordereddb: not found")
	ErrClosed          = errors.New("ordereddb: closed")
)

type BlockNumber uint64

type NodeHash [NodeHashSize]byte

type NodeKey [NodeKeySize]byte

func NewNodeHash(hash []byte) (NodeHash, error) {
	var out NodeHash
	if len(hash) != NodeHashSize {
		return out, fmt.Errorf("%w: got %d bytes, want %d", ErrInvalidNodeHash, len(hash), NodeHashSize)
	}
	copy(out[:], hash)
	return out, nil
}

func MustNodeHash(hash []byte) NodeHash {
	out, err := NewNodeHash(hash)
	if err != nil {
		panic(err)
	}
	return out
}

func NewNodeKey(block BlockNumber, hash NodeHash) NodeKey {
	var key NodeKey
	binary.BigEndian.PutUint64(key[:BlockNumberSize], uint64(block))
	copy(key[BlockNumberSize:], hash[:])
	return key
}

func ParseNodeKey(raw []byte) (BlockNumber, NodeHash, error) {
	var hash NodeHash
	if len(raw) != NodeKeySize {
		return 0, hash, fmt.Errorf("%w: got %d bytes, want %d", ErrInvalidNodeKey, len(raw), NodeKeySize)
	}
	block := BlockNumber(binary.BigEndian.Uint64(raw[:BlockNumberSize]))
	copy(hash[:], raw[BlockNumberSize:])
	return block, hash, nil
}

func (k NodeKey) BlockNumber() BlockNumber {
	return BlockNumber(binary.BigEndian.Uint64(k[:BlockNumberSize]))
}

func (k NodeKey) Hash() NodeHash {
	var hash NodeHash
	copy(hash[:], k[BlockNumberSize:])
	return hash
}

func (k NodeKey) Bytes() []byte {
	out := make([]byte, NodeKeySize)
	copy(out, k[:])
	return out
}

func (h NodeHash) Bytes() []byte {
	out := make([]byte, NodeHashSize)
	copy(out, h[:])
	return out
}

func (h NodeHash) String() string {
	return hex.EncodeToString(h[:])
}
