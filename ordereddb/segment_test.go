package ordereddb

import (
	"bytes"
	"errors"
	"testing"
)

func TestSegmentWriteRead(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewSegmentWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	hashA := testHash(1)
	hashB := testHash(2)
	hashC := testHash(3)
	entries := []SegmentEntry{
		{Key: NewNodeKey(10, hashA), Value: []byte("a")},
		{Key: NewNodeKey(10, hashB), Value: []byte("b")},
		{Key: NewNodeKey(11, hashC), Value: []byte("c")},
	}
	for _, entry := range entries {
		if err := writer.Append(entry); err != nil {
			t.Fatal(err)
		}
	}
	meta, err := writer.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if meta.FirstBlock != 10 || meta.LastBlock != 11 {
		t.Fatalf("unexpected range %d:%d", meta.FirstBlock, meta.LastBlock)
	}
	reader, err := NewSegmentReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	value, err := reader.GetNode(10, hashB)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "b" {
		t.Fatalf("unexpected value %q", value)
	}
	_, err = reader.GetNode(12, hashA)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSegmentRejectsUnsortedEntries(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewSegmentWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Append(SegmentEntry{Key: NewNodeKey(2, testHash(1)), Value: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	err = writer.Append(SegmentEntry{Key: NewNodeKey(1, testHash(1)), Value: []byte("b")})
	if !errors.Is(err, ErrUnsortedSegment) {
		t.Fatalf("expected ErrUnsortedSegment, got %v", err)
	}
}

func TestSegmentRejectsChecksumMismatch(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewSegmentWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Append(SegmentEntry{Key: NewNodeKey(1, testHash(1)), Value: []byte("a")}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Finish(); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	raw[segmentHeaderSize+NodeKeySize+4] ^= 0xff
	_, err = NewSegmentReader(bytes.NewReader(raw), int64(len(raw)))
	if !errors.Is(err, ErrInvalidSegment) {
		t.Fatalf("expected ErrInvalidSegment, got %v", err)
	}
}

func TestSegmentBlockLocalHashIndex(t *testing.T) {
	var buf bytes.Buffer
	writer, err := NewSegmentWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for i := byte(1); i <= 5; i++ {
		if err := writer.Append(SegmentEntry{
			Key:   NewNodeKey(3, testHash(i)),
			Value: []byte{i},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := writer.Finish(); err != nil {
		t.Fatal(err)
	}
	reader, err := NewSegmentReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	value, err := reader.GetNode(3, testHash(5))
	if err != nil {
		t.Fatal(err)
	}
	if len(value) != 1 || value[0] != 5 {
		t.Fatalf("unexpected value %v", value)
	}
}

func testHash(seed byte) NodeHash {
	var hash NodeHash
	for i := range hash {
		hash[i] = seed
	}
	return hash
}
