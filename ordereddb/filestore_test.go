package ordereddb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreSealAndReadSegment(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{SegmentMaxBlocks: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hashA := testHash(10)
	hashB := testHash(11)
	root := testHash(99)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hashA, []byte("node-a")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(2, hashB, []byte("node-b")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(2, root); err != nil {
		t.Fatal(err)
	}
	meta, err := store.SealThrough(2)
	if err != nil {
		t.Fatal(err)
	}
	if meta.FirstBlock != 1 || meta.LastBlock != 2 {
		t.Fatalf("unexpected segment range %d:%d", meta.FirstBlock, meta.LastBlock)
	}
	value, err := store.GetNode(2, hashB)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "node-b" {
		t.Fatalf("unexpected value %q", value)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenFileStore(dir, Options{SegmentMaxBlocks: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	value, err = reopened.GetNode(1, hashA)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "node-a" {
		t.Fatalf("unexpected reopened value %q", value)
	}
	_, err = store.GetNode(3, hashB)
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestFileStoreRecoversCommittedHotBlock(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	hash := testHash(22)
	root := testHash(23)
	if err := store.BeginBlock(7, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(7, hash, []byte("hot-node")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(7, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	value, err := reopened.GetNode(7, hash)
	if err != nil {
		t.Fatal(err)
	}
	if string(value) != "hot-node" {
		t.Fatalf("unexpected value %q", value)
	}
}

func TestFileStoreTruncatesPartialHotJournalTail(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	hashA := testHash(121)
	hashB := testHash(122)
	root := testHash(123)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hashA, []byte("keep")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, hotJournalFileName)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte{1, 2, 3, 4}); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.GetNode(1, hashA); err != nil {
		t.Fatal(err)
	}
	_, err = reopened.GetNode(2, hashB)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStoreRejectsCorruptHotJournalRecord(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	hash := testHash(124)
	root := testHash(125)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hash, []byte("node")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, hotJournalFileName)
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{0xff}, hotJournalHeaderSize); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = OpenFileStore(dir, Options{})
	if !errors.Is(err, ErrInvalidSegment) {
		t.Fatalf("expected ErrInvalidSegment, got %v", err)
	}
}

func TestFileStoreRollbackRewritesHotJournal(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	root := testHash(31)
	hashA := testHash(32)
	hashB := testHash(33)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hashA, []byte("keep")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(2, hashB, []byte("drop")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if err := store.RollbackTo(1); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.GetNode(1, hashA); err != nil {
		t.Fatal(err)
	}
	_, err = reopened.GetNode(2, hashB)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileStoreRejectsCorruptSegmentOnOpen(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	hash := testHash(41)
	root := testHash(42)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hash, []byte("node")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	meta, err := store.SealThrough(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "segments", meta.FileName)
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte{0xff}, segmentHeaderSize+NodeKeySize+4); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = OpenFileStore(dir, Options{})
	if !errors.Is(err, ErrInvalidSegment) {
		t.Fatalf("expected ErrInvalidSegment, got %v", err)
	}
}

func TestFileStoreCleansTempSegmentsOnOpen(t *testing.T) {
	dir := t.TempDir()
	segDir := filepath.Join(dir, "segments")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(segDir, ".state-000.tmp")
	if err := os.WriteFile(tmp, []byte("partial"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := os.Stat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp segment cleanup, got %v", err)
	}
}

func TestFileStoreSegmentCacheHonorsLimit(t *testing.T) {
	store, err := OpenFileStore(t.TempDir(), Options{SegmentMaxBlocks: 1, SegmentCacheSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := testHash(51)
	hashA := testHash(52)
	hashB := testHash(53)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hashA, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SealThrough(1); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(2, hashB, []byte("b")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SealThrough(2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNode(1, hashA); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNode(2, hashB); err != nil {
		t.Fatal(err)
	}
	if len(store.segmentCache) > 1 {
		t.Fatalf("cache exceeded limit: %d", len(store.segmentCache))
	}
}

func TestFileStoreRootIndexSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	root := testHash(81)
	hash := testHash(82)
	if err := store.BeginBlock(9, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(9, hash, []byte("node")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(9, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Canonicalize(9, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenFileStore(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	meta, err := reopened.ResolveRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if meta.Block != 9 || !meta.Canonical {
		t.Fatalf("unexpected root meta %+v", meta)
	}
}

func TestFileStoreStatsTrackDuplicateAndCache(t *testing.T) {
	store, err := OpenFileStore(t.TempDir(), Options{SegmentMaxBlocks: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	root := testHash(91)
	hash := testHash(92)
	if err := store.BeginBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(1, hash, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(1, root); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if err := store.PutNode(2, hash, []byte("a")); err != nil {
		t.Fatal(err)
	}
	if err := store.CommitBlock(2, root); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SealThrough(2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNode(1, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetNode(1, hash); err != nil {
		t.Fatal(err)
	}
	stats := store.Stats()
	if stats.DuplicateNodeHits == 0 {
		t.Fatalf("expected duplicate node hit, stats %+v", stats)
	}
	if stats.SegmentCacheMisses == 0 || stats.SegmentCacheHits == 0 {
		t.Fatalf("expected segment cache hit/miss, stats %+v", stats)
	}
}
