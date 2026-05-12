package ordereddb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const manifestFileName = "MANIFEST.json"

type fileManifest struct {
	NextSegmentID uint64        `json:"next_segment_id"`
	Blocks        []BlockMeta   `json:"blocks"`
	Segments      []SegmentMeta `json:"segments"`
	Roots         []RootMeta    `json:"roots"`
}

func (s *FileStore) loadManifest() error {
	path := filepath.Join(s.dir, manifestFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var disk fileManifest
	if err := json.Unmarshal(data, &disk); err != nil {
		return err
	}
	s.nextSegmentID = disk.NextSegmentID
	for _, block := range disk.Blocks {
		if err := s.manifest.PutBlock(block); err != nil {
			return err
		}
	}
	for _, segment := range disk.Segments {
		if err := s.manifest.PutSegment(segment); err != nil {
			return err
		}
		s.segments[segment.ID] = segment
		if segment.ID >= s.nextSegmentID {
			s.nextSegmentID = segment.ID + 1
		}
	}
	for _, root := range disk.Roots {
		if err := s.manifest.RecordStateRoot(root.Block, root.Root, root.Canonical); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) persistManifestLocked() error {
	blocks, segments, roots := s.manifest.Snapshot()
	disk := fileManifest{
		NextSegmentID: s.nextSegmentID,
		Blocks:        blocks,
		Segments:      segments,
		Roots:         roots,
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	s.stats.ManifestBytes = uint64(len(data))
	path := filepath.Join(s.dir, manifestFileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDir(s.dir)
}

func (s *FileStore) cleanupTempSegments() error {
	entries, err := os.ReadDir(s.segDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp") {
			if err := os.Remove(filepath.Join(s.segDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (s *FileStore) validateSegments() error {
	for _, meta := range s.segments {
		if meta.Status == SegmentDeleted {
			continue
		}
		path := filepath.Join(s.segDir, meta.FileName)
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open segment %s: %w", meta.FileName, err)
		}
		info, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return err
		}
		reader, err := NewSegmentReader(file, info.Size())
		_ = file.Close()
		if err != nil {
			return fmt.Errorf("validate segment %s: %w", meta.FileName, err)
		}
		actual := reader.Meta()
		if actual.FirstBlock != meta.FirstBlock || actual.LastBlock != meta.LastBlock || actual.Checksum != meta.Checksum {
			return fmt.Errorf("%w: segment metadata mismatch for %s", ErrInvalidSegment, meta.FileName)
		}
	}
	return nil
}
