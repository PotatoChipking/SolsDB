package ordereddb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
)

const (
	segmentVersion        = uint32(1)
	segmentHeaderSize     = 16
	segmentIndexEntrySize = 40
	segmentNodeIndexSize  = NodeHashSize + 16
	segmentFooterSize     = 48
)

var (
	segmentMagic       = [8]byte{'S', 'O', 'L', 'S', 'S', 'E', 'G', '1'}
	segmentFooterMagic = [8]byte{'S', 'O', 'L', 'S', 'F', 'T', 'R', '1'}

	ErrInvalidSegment  = errors.New("ordereddb: invalid segment")
	ErrUnsortedSegment = errors.New("ordereddb: segment entries must be sorted")
)

type SegmentEntry struct {
	Key   NodeKey
	Value []byte
}

type SegmentIndexEntry struct {
	Block           BlockNumber
	FirstOffset     uint64
	LastOffset      uint64
	NodeIndexOffset uint64
	NodeCount       uint64
}

type segmentNodeIndexEntry struct {
	Hash        NodeHash
	ValueOffset uint64
	ValueLen    uint64
}

type SegmentWriter struct {
	w            io.Writer
	offset       uint64
	crc          hash32
	lastKey      NodeKey
	hasLast      bool
	firstBlock   BlockNumber
	lastBlock    BlockNumber
	index        []SegmentIndexEntry
	nodeIndexes  map[BlockNumber][]segmentNodeIndexEntry
	openBlock    BlockNumber
	openOffset   uint64
	hasOpenBlock bool
	closed       bool
}

type hash32 interface {
	io.Writer
	Sum32() uint32
}

func NewSegmentWriter(w io.Writer) (*SegmentWriter, error) {
	sw := &SegmentWriter{
		w:           w,
		crc:         crc32.NewIEEE(),
		nodeIndexes: make(map[BlockNumber][]segmentNodeIndexEntry),
	}
	header := make([]byte, segmentHeaderSize)
	copy(header[:8], segmentMagic[:])
	binary.BigEndian.PutUint32(header[8:12], segmentVersion)
	if err := sw.writeRaw(header, false); err != nil {
		return nil, err
	}
	return sw, nil
}

func (w *SegmentWriter) Append(entry SegmentEntry) error {
	if w.closed {
		return ErrClosed
	}
	if len(entry.Value) == 0 {
		return fmt.Errorf("%w: empty value", ErrInvalidSegment)
	}
	if w.hasLast && bytes.Compare(entry.Key[:], w.lastKey[:]) <= 0 {
		return ErrUnsortedSegment
	}
	block := entry.Key.BlockNumber()
	if !w.hasLast {
		w.firstBlock = block
	}
	if !w.hasOpenBlock || block != w.openBlock {
		w.closeOpenBlock()
		w.openBlock = block
		w.openOffset = w.offset
		w.hasOpenBlock = true
	}
	record := make([]byte, NodeKeySize+4+len(entry.Value))
	copy(record[:NodeKeySize], entry.Key[:])
	binary.BigEndian.PutUint32(record[NodeKeySize:NodeKeySize+4], uint32(len(entry.Value)))
	copy(record[NodeKeySize+4:], entry.Value)
	valueOffset := w.offset + NodeKeySize + 4
	if err := w.writeRaw(record, true); err != nil {
		return err
	}
	w.nodeIndexes[block] = append(w.nodeIndexes[block], segmentNodeIndexEntry{
		Hash:        entry.Key.Hash(),
		ValueOffset: valueOffset,
		ValueLen:    uint64(len(entry.Value)),
	})
	w.lastKey = entry.Key
	w.lastBlock = block
	w.hasLast = true
	return nil
}

func (w *SegmentWriter) Finish() (SegmentMeta, error) {
	if w.closed {
		return SegmentMeta{}, ErrClosed
	}
	w.closeOpenBlock()
	nodeIndexOffsets := make(map[BlockNumber]uint64, len(w.index))
	for _, entry := range w.index {
		nodeIndexOffsets[entry.Block] = w.offset
		for _, node := range w.nodeIndexes[entry.Block] {
			buf := make([]byte, segmentNodeIndexSize)
			copy(buf[:NodeHashSize], node.Hash[:])
			binary.BigEndian.PutUint64(buf[NodeHashSize:NodeHashSize+8], node.ValueOffset)
			binary.BigEndian.PutUint64(buf[NodeHashSize+8:NodeHashSize+16], node.ValueLen)
			if err := w.writeRaw(buf, true); err != nil {
				return SegmentMeta{}, err
			}
		}
	}
	indexOffset := w.offset
	for _, entry := range w.index {
		buf := make([]byte, segmentIndexEntrySize)
		binary.BigEndian.PutUint64(buf[0:8], uint64(entry.Block))
		binary.BigEndian.PutUint64(buf[8:16], entry.FirstOffset)
		binary.BigEndian.PutUint64(buf[16:24], entry.LastOffset)
		binary.BigEndian.PutUint64(buf[24:32], nodeIndexOffsets[entry.Block])
		binary.BigEndian.PutUint64(buf[32:40], uint64(len(w.nodeIndexes[entry.Block])))
		if err := w.writeRaw(buf, true); err != nil {
			return SegmentMeta{}, err
		}
	}
	checksum := w.crc.Sum32()
	footer := make([]byte, segmentFooterSize)
	binary.BigEndian.PutUint64(footer[0:8], indexOffset)
	binary.BigEndian.PutUint64(footer[8:16], uint64(len(w.index)))
	binary.BigEndian.PutUint64(footer[16:24], uint64(w.firstBlock))
	binary.BigEndian.PutUint64(footer[24:32], uint64(w.lastBlock))
	binary.BigEndian.PutUint32(footer[32:36], checksum)
	copy(footer[40:48], segmentFooterMagic[:])
	if err := w.writeRaw(footer, false); err != nil {
		return SegmentMeta{}, err
	}
	w.closed = true
	return SegmentMeta{
		FirstBlock: w.firstBlock,
		LastBlock:  w.lastBlock,
		Size:       w.offset,
		Checksum:   checksum,
		Status:     SegmentSealed,
	}, nil
}

func (w *SegmentWriter) closeOpenBlock() {
	if !w.hasOpenBlock {
		return
	}
	w.index = append(w.index, SegmentIndexEntry{
		Block:       w.openBlock,
		FirstOffset: w.openOffset,
		LastOffset:  w.offset,
	})
	w.hasOpenBlock = false
}

func (w *SegmentWriter) writeRaw(data []byte, checksum bool) error {
	if _, err := w.w.Write(data); err != nil {
		return err
	}
	if checksum {
		if _, err := w.crc.Write(data); err != nil {
			return err
		}
	}
	w.offset += uint64(len(data))
	return nil
}

type SegmentReader struct {
	r     io.ReaderAt
	size  int64
	meta  SegmentMeta
	index []SegmentIndexEntry
}

func NewSegmentReader(r io.ReaderAt, size int64) (*SegmentReader, error) {
	if size < segmentHeaderSize+segmentFooterSize {
		return nil, ErrInvalidSegment
	}
	header := make([]byte, segmentHeaderSize)
	if _, err := r.ReadAt(header, 0); err != nil {
		return nil, err
	}
	if !bytes.Equal(header[:8], segmentMagic[:]) {
		return nil, ErrInvalidSegment
	}
	if version := binary.BigEndian.Uint32(header[8:12]); version != segmentVersion {
		return nil, fmt.Errorf("%w: version %d", ErrInvalidSegment, version)
	}
	footer := make([]byte, segmentFooterSize)
	if _, err := r.ReadAt(footer, size-segmentFooterSize); err != nil {
		return nil, err
	}
	if !bytes.Equal(footer[40:48], segmentFooterMagic[:]) {
		return nil, ErrInvalidSegment
	}
	indexOffset := binary.BigEndian.Uint64(footer[0:8])
	indexCount := binary.BigEndian.Uint64(footer[8:16])
	firstBlock := BlockNumber(binary.BigEndian.Uint64(footer[16:24]))
	lastBlock := BlockNumber(binary.BigEndian.Uint64(footer[24:32]))
	checksum := binary.BigEndian.Uint32(footer[32:36])
	indexSize := indexCount * segmentIndexEntrySize
	if indexOffset < segmentHeaderSize || indexOffset+indexSize > uint64(size-segmentFooterSize) {
		return nil, ErrInvalidSegment
	}
	if err := verifySegmentChecksum(r, indexOffset+indexSize, checksum); err != nil {
		return nil, err
	}
	indexBuf := make([]byte, indexSize)
	if _, err := r.ReadAt(indexBuf, int64(indexOffset)); err != nil {
		return nil, err
	}
	index := make([]SegmentIndexEntry, indexCount)
	for i := range index {
		base := i * segmentIndexEntrySize
		index[i] = SegmentIndexEntry{
			Block:           BlockNumber(binary.BigEndian.Uint64(indexBuf[base : base+8])),
			FirstOffset:     binary.BigEndian.Uint64(indexBuf[base+8 : base+16]),
			LastOffset:      binary.BigEndian.Uint64(indexBuf[base+16 : base+24]),
			NodeIndexOffset: binary.BigEndian.Uint64(indexBuf[base+24 : base+32]),
			NodeCount:       binary.BigEndian.Uint64(indexBuf[base+32 : base+40]),
		}
	}
	return &SegmentReader{
		r:    r,
		size: size,
		meta: SegmentMeta{
			FirstBlock: firstBlock,
			LastBlock:  lastBlock,
			Size:       uint64(size),
			Checksum:   checksum,
			Status:     SegmentSealed,
		},
		index: index,
	}, nil
}

func verifySegmentChecksum(r io.ReaderAt, end uint64, expected uint32) error {
	if end < segmentHeaderSize {
		return ErrInvalidSegment
	}
	crc := crc32.NewIEEE()
	offset := uint64(segmentHeaderSize)
	buf := make([]byte, 64*1024)
	for offset < end {
		n := uint64(len(buf))
		if remaining := end - offset; remaining < n {
			n = remaining
		}
		chunk := buf[:n]
		if _, err := r.ReadAt(chunk, int64(offset)); err != nil {
			return err
		}
		if _, err := crc.Write(chunk); err != nil {
			return err
		}
		offset += n
	}
	if got := crc.Sum32(); got != expected {
		return fmt.Errorf("%w: checksum mismatch got %08x want %08x", ErrInvalidSegment, got, expected)
	}
	return nil
}

func (r *SegmentReader) Meta() SegmentMeta {
	return r.meta
}

func (r *SegmentReader) GetNode(block BlockNumber, hash NodeHash) ([]byte, error) {
	if block < r.meta.FirstBlock || block > r.meta.LastBlock {
		return nil, ErrNotFound
	}
	pos := sort.Search(len(r.index), func(i int) bool {
		return r.index[i].Block >= block
	})
	if pos == len(r.index) || r.index[pos].Block != block {
		return nil, ErrNotFound
	}
	target := NewNodeKey(block, hash)
	if r.index[pos].NodeCount > 0 {
		return r.lookupBlockIndex(r.index[pos], target.Hash())
	}
	return r.scanRange(r.index[pos], target)
}

func (r *SegmentReader) lookupBlockIndex(index SegmentIndexEntry, hash NodeHash) ([]byte, error) {
	buf := make([]byte, segmentNodeIndexSize)
	for i := uint64(0); i < index.NodeCount; i++ {
		offset := index.NodeIndexOffset + i*segmentNodeIndexSize
		if _, err := r.r.ReadAt(buf, int64(offset)); err != nil {
			return nil, err
		}
		cmp := bytes.Compare(buf[:NodeHashSize], hash[:])
		if cmp == 0 {
			valueOffset := binary.BigEndian.Uint64(buf[NodeHashSize : NodeHashSize+8])
			valueLen := binary.BigEndian.Uint64(buf[NodeHashSize+8 : NodeHashSize+16])
			value := make([]byte, valueLen)
			if _, err := r.r.ReadAt(value, int64(valueOffset)); err != nil {
				return nil, err
			}
			return value, nil
		}
		if cmp > 0 {
			return nil, ErrNotFound
		}
	}
	return nil, ErrNotFound
}

func (r *SegmentReader) scanRange(index SegmentIndexEntry, target NodeKey) ([]byte, error) {
	offset := index.FirstOffset
	header := make([]byte, NodeKeySize+4)
	for offset < index.LastOffset {
		if _, err := r.r.ReadAt(header, int64(offset)); err != nil {
			return nil, err
		}
		var key NodeKey
		copy(key[:], header[:NodeKeySize])
		valueLen := binary.BigEndian.Uint32(header[NodeKeySize : NodeKeySize+4])
		valueOffset := offset + NodeKeySize + 4
		nextOffset := valueOffset + uint64(valueLen)
		cmp := bytes.Compare(key[:], target[:])
		if cmp == 0 {
			value := make([]byte, valueLen)
			if _, err := r.r.ReadAt(value, int64(valueOffset)); err != nil {
				return nil, err
			}
			return value, nil
		}
		if cmp > 0 {
			return nil, ErrNotFound
		}
		offset = nextOffset
	}
	return nil, ErrNotFound
}
