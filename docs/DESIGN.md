# SolsDB Design

## Problem

Ethereum clients commonly store trie nodes in a general-purpose key/value
database. This keeps the storage interface simple, but state trie reads and
writes are mixed with other chain data and inherit the write amplification and
lookup behavior of a generic LSM tree.

SolsDB separates state trie node storage into an ordered, block-aware path while
leaving ordinary chain data compatible with existing key/value storage.

## Core Key

State trie nodes are addressed by:

```text
8-byte big-endian block number || 32-byte trie node hash
```

The block prefix makes lookup deterministic when the caller knows which state
root or block context it is reading from.

SolsDB also records a root index:

```text
stateRoot -> blockNumber
blockNumber -> stateRoot
```

This index gives root-based callers an explicit way to recover block context.

## Write Path

The intended write lifecycle is:

```text
BeginBlock(block, root)
PutNode(block, hash, rlp)
PutNode(block, hash, rlp)
CommitBlock(block, root)
```

`CommitBlock` is the durability boundary. A crash before the commit marker must
roll back the partial block. A crash after the marker must recover the block as
committed.

## Storage Files

SolsDB stores committed data in immutable range segments. A segment covers a
bounded consecutive block range, such as 4096 blocks or a size-limited range.
This avoids the file-count growth caused by creating one file per block.

Example:

```text
segments/state-00000000-00004095.sdb
segments/state-00004096-00008191.sdb
segments/state-00008192-00012287.sdb
```

Each segment contains sorted `NodeKey -> RLP` entries. Segment metadata must
include:

- segment identifier
- first and last block
- file name
- file size
- checksum
- lifecycle status

Block metadata is tracked separately from segment metadata. A block has a state
root and lifecycle status; a segment is a physical file that may contain many
blocks.

Segments include two index layers:

- block index: `blockNumber -> node index range`
- block-local node index: `nodeHash -> value offset`

This avoids scanning all trie nodes for a block on every lookup.

## Hot Layer and Sealing

Recent blocks are written into a hot mutable layer. The hot layer is sealed into
an immutable segment when one of these limits is reached:

- configured block count
- configured byte size
- finalized block boundary
- clean shutdown flush

The default design target is to keep the reorg window in the hot layer and seal
only finalized or sufficiently old blocks.

## Segment Cache

Segment readers are cached by segment id to avoid reopening and revalidating the
same immutable file on every trie node lookup. The default cache size is 64
segments. Eviction closes the underlying segment file.

## Metrics

`FileStore.Stats` exposes counters used to interpret benchmarks and integration
tests:

- put/get node counts
- duplicate node observations
- hot journal bytes
- manifest bytes
- sealed segment count
- segment cache hit/miss counts

## Read Path

Reads check hot data first, then the manifest-backed block table index:

```text
GetNode(block, hash)
  -> hot block writer buffer
  -> manifest range lookup
  -> immutable segment
  -> ErrNotFound
```

The implementation must not rely on hard-coded block numbers or inferred file
names alone.

## Recovery

Recovery scans the manifest and hot block commit journal:

1. Load `MANIFEST.json` to discover sealed segments and block metadata.
2. Replay `HOTLOG.jsonl` to restore committed blocks that have not been sealed.
3. Ignore hot journal records for blocks already covered by sealed segments.
4. Verify sealed segment checksums.
5. Rebuild in-memory segment indexes.
6. Restore the last committed block from metadata.

`CommitBlock` appends the block payload to the binary hot journal before
exposing it as committed hot data. Each hot journal record contains a magic,
version, payload length, and CRC32. Startup truncates an incomplete tail record
and rejects records with checksum mismatches. When hot data is sealed into a
segment, the hot journal is rewritten to keep only the remaining unsealed blocks.

Segment sealing follows this durability order:

1. Write the segment to a temporary file.
2. Finish the footer and checksum.
3. Sync and close the segment file.
4. Rename the temporary file to its final segment name.
5. Sync the segment directory.
6. Update `MANIFEST.json` through a temporary file and atomic rename.
7. Rewrite `HOTLOG.jsonl` to remove blocks now covered by the sealed segment.

Startup removes stale temporary segment files before validating manifest entries.

## Reorg and Pruning

Reorg and pruning are explicit lifecycle transitions:

- `Canonicalize` marks the canonical block root.
- `RollbackTo` removes pending/canonical metadata above a block.
- `PruneBefore` marks older segments eligible for deletion.

Physical deletion is delayed until no active reader references the file.
