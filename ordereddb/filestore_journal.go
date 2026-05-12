package ordereddb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	hotJournalFileName   = "HOTLOG.bin"
	hotJournalVersion    = uint32(1)
	hotJournalHeaderSize = 24
)

var hotJournalMagic = [8]byte{'S', 'O', 'L', 'S', 'J', 'N', 'L', '1'}

type hotJournalRecord struct {
	Block BlockNumber
	Root  NodeHash
	Nodes []hotJournalNode
}

type hotJournalNode struct {
	Hash  NodeHash
	Value []byte
}

func (s *FileStore) loadHotJournal() error {
	path := filepath.Join(s.dir, hotJournalFileName)
	file, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()
	var offset int64
	for {
		record, next, err := readHotJournalRecord(file, offset)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				if truncErr := file.Truncate(offset); truncErr != nil {
					return truncErr
				}
				return nil
			}
			return err
		}
		offset = next
		if _, err := s.manifest.FindSegment(record.Block); err == nil {
			continue
		}
		nodes := make(map[NodeHash][]byte, len(record.Nodes))
		for _, node := range record.Nodes {
			nodes[node.Hash] = cloneBytes(node.Value)
			s.seenNodes[node.Hash] = struct{}{}
		}
		s.hot[record.Block] = nodes
		s.roots[record.Block] = record.Root
		if err := s.manifest.PutBlock(BlockMeta{
			Number:    record.Block,
			Root:      record.Root,
			Status:    BlockCommitted,
			NodeCount: uint64(len(nodes)),
		}); err != nil {
			return err
		}
	}
}

func (s *FileStore) appendHotJournalLocked(block BlockNumber, root NodeHash, nodes map[NodeHash][]byte) error {
	record := hotJournalRecord{
		Block: block,
		Root:  root,
		Nodes: make([]hotJournalNode, 0, len(nodes)),
	}
	hashes := make([]NodeHash, 0, len(nodes))
	for hash := range nodes {
		hashes = append(hashes, hash)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i][:], hashes[j][:]) < 0
	})
	for _, hash := range hashes {
		record.Nodes = append(record.Nodes, hotJournalNode{
			Hash:  hash,
			Value: cloneBytes(nodes[hash]),
		})
	}
	data, err := encodeHotJournalRecord(record)
	if err != nil {
		return err
	}
	s.stats.HotJournalBytes += uint64(len(data))
	path := filepath.Join(s.dir, hotJournalFileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (s *FileStore) rewriteHotJournalLocked() error {
	path := filepath.Join(s.dir, hotJournalFileName)
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	blocks := make([]BlockNumber, 0, len(s.hot))
	for block := range s.hot {
		blocks = append(blocks, block)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i] < blocks[j] })
	for _, block := range blocks {
		root := s.roots[block]
		record := hotJournalRecord{
			Block: block,
			Root:  root,
			Nodes: make([]hotJournalNode, 0, len(s.hot[block])),
		}
		hashes := make([]NodeHash, 0, len(s.hot[block]))
		for hash := range s.hot[block] {
			hashes = append(hashes, hash)
		}
		sort.Slice(hashes, func(i, j int) bool {
			return bytes.Compare(hashes[i][:], hashes[j][:]) < 0
		})
		for _, hash := range hashes {
			record.Nodes = append(record.Nodes, hotJournalNode{
				Hash:  hash,
				Value: cloneBytes(s.hot[block][hash]),
			})
		}
		data, err := encodeHotJournalRecord(record)
		if err != nil {
			_ = file.Close()
			return err
		}
		if _, err := file.Write(data); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(s.dir)
}

func encodeHotJournalRecord(record hotJournalRecord) ([]byte, error) {
	payload := new(bytes.Buffer)
	if err := binary.Write(payload, binary.BigEndian, uint64(record.Block)); err != nil {
		return nil, err
	}
	if _, err := payload.Write(record.Root[:]); err != nil {
		return nil, err
	}
	if len(record.Nodes) > int(^uint32(0)) {
		return nil, fmt.Errorf("%w: too many journal nodes", ErrInvalidSegment)
	}
	if err := binary.Write(payload, binary.BigEndian, uint32(len(record.Nodes))); err != nil {
		return nil, err
	}
	for _, node := range record.Nodes {
		if _, err := payload.Write(node.Hash[:]); err != nil {
			return nil, err
		}
		if len(node.Value) > int(^uint32(0)) {
			return nil, fmt.Errorf("%w: journal value too large", ErrInvalidSegment)
		}
		if err := binary.Write(payload, binary.BigEndian, uint32(len(node.Value))); err != nil {
			return nil, err
		}
		if _, err := payload.Write(node.Value); err != nil {
			return nil, err
		}
	}
	body := payload.Bytes()
	header := make([]byte, hotJournalHeaderSize)
	copy(header[:8], hotJournalMagic[:])
	binary.BigEndian.PutUint32(header[8:12], hotJournalVersion)
	binary.BigEndian.PutUint32(header[12:16], uint32(len(body)))
	binary.BigEndian.PutUint32(header[16:20], crc32.ChecksumIEEE(body))
	return append(header, body...), nil
}

func readHotJournalRecord(r io.ReaderAt, offset int64) (hotJournalRecord, int64, error) {
	header := make([]byte, hotJournalHeaderSize)
	if _, err := r.ReadAt(header, offset); err != nil {
		return hotJournalRecord{}, offset, err
	}
	if !bytes.Equal(header[:8], hotJournalMagic[:]) {
		return hotJournalRecord{}, offset, fmt.Errorf("%w: invalid hot journal magic", ErrInvalidSegment)
	}
	if version := binary.BigEndian.Uint32(header[8:12]); version != hotJournalVersion {
		return hotJournalRecord{}, offset, fmt.Errorf("%w: hot journal version %d", ErrInvalidSegment, version)
	}
	length := binary.BigEndian.Uint32(header[12:16])
	checksum := binary.BigEndian.Uint32(header[16:20])
	payload := make([]byte, length)
	if _, err := r.ReadAt(payload, offset+hotJournalHeaderSize); err != nil {
		return hotJournalRecord{}, offset, err
	}
	if got := crc32.ChecksumIEEE(payload); got != checksum {
		return hotJournalRecord{}, offset, fmt.Errorf("%w: hot journal checksum mismatch", ErrInvalidSegment)
	}
	record, err := decodeHotJournalPayload(payload)
	if err != nil {
		return hotJournalRecord{}, offset, err
	}
	return record, offset + hotJournalHeaderSize + int64(length), nil
}

func decodeHotJournalPayload(payload []byte) (hotJournalRecord, error) {
	var record hotJournalRecord
	if len(payload) < 8+NodeHashSize+4 {
		return record, io.ErrUnexpectedEOF
	}
	record.Block = BlockNumber(binary.BigEndian.Uint64(payload[:8]))
	copy(record.Root[:], payload[8:8+NodeHashSize])
	pos := 8 + NodeHashSize
	nodeCount := binary.BigEndian.Uint32(payload[pos : pos+4])
	pos += 4
	record.Nodes = make([]hotJournalNode, 0, nodeCount)
	for i := uint32(0); i < nodeCount; i++ {
		if len(payload)-pos < NodeHashSize+4 {
			return record, io.ErrUnexpectedEOF
		}
		var node hotJournalNode
		copy(node.Hash[:], payload[pos:pos+NodeHashSize])
		pos += NodeHashSize
		valueLen := int(binary.BigEndian.Uint32(payload[pos : pos+4]))
		pos += 4
		if len(payload)-pos < valueLen {
			return record, io.ErrUnexpectedEOF
		}
		node.Value = cloneBytes(payload[pos : pos+valueLen])
		pos += valueLen
		record.Nodes = append(record.Nodes, node)
	}
	if pos != len(payload) {
		return record, fmt.Errorf("%w: trailing hot journal payload bytes", ErrInvalidSegment)
	}
	return record, nil
}
