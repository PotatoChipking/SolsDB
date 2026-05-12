# SolsDB

SolsDB is an experimental single-level ordered log-structured storage engine for
Ethereum state trie data.

The project goal is to make Merkle Patricia Trie node storage block-aware:
state trie nodes are written in block order and can be read through a stable
`blockNumber + nodeHash` address. This repository is intended to become a clean
reference implementation for Ethereum integration work and pull request
discussion.

## Status

This repository contains the standalone SolsDB storage prototype, a lightweight
Geth-facing adapter, recovery logic, segment files, tests, benchmark scaffolding,
and Geth integration documents.

The project is still experimental. The next major milestone is a minimal patch
against go-ethereum `v1.17.2` that proves the state trie commit path can pass
block context into SolsDB.

## Why SolsDB

Geth commonly stores Ethereum state trie nodes in LevelDB as `nodeHash -> RLP`.
This keeps the interface simple, but it loses useful state timeline information.
Because node hashes are effectively random, state nodes from many blocks are
mixed across LSM levels. As data grows, reads may search multiple SSTables and
writes may trigger compaction, increasing read/write amplification.

SolsDB stores state trie data with explicit block context:

```text
blockNumber + nodeHash -> rlpNode
```

This lets the engine organize state data in block order, seal committed state
nodes into immutable range segments, and locate the relevant segment directly
when a caller has block or state-root context.

## Design Goals

- Keep ordinary chain data compatible with existing Ethereum key/value storage.
- Store state trie nodes through an ordered, block-aware path.
- Group committed blocks into bounded range segments instead of creating one
  file per block.
- Make crash recovery explicit with block commit markers.
- Support reorg and pruning as first-class storage lifecycle operations.
- Provide a small adapter surface for Geth integration.

## Architecture

SolsDB separates Ethereum storage into two paths:

```text
headers / bodies / receipts / metadata -> existing Ethereum key/value backend
state trie node RLPs                  -> SolsDB ordered state backend
```

The ordered state backend has four main layers:

- Hot layer: committed but unsealed recent blocks, protected by `HOTLOG.bin`.
- Segment layer: immutable `.sdb` files covering bounded block ranges.
- Manifest: `MANIFEST.json` records block metadata, segment metadata, and
  `stateRoot -> blockNumber`.
- Adapter: `ethadapter` exposes a Geth-like key/value surface plus explicit
  state-node methods such as `PutStateNode(block, hash, rlp)`.

## Repository Layout

- `ordereddb/`: public storage model and codec primitives.
- `ethadapter/`: dependency-light Geth-facing adapter.
- `examples/geth-wrapper/`: minimal composition example.
- `docs/DESIGN.md`: storage architecture.
- `docs/INTEGRATION.md`: Ethereum and Geth integration plan.
- `docs/GETH_BASELINE.md`: pinned Geth version for integration work.
- `docs/GETH_PATCH.md`: proposed Geth-side patch plan.
- `docs/EPF7_PROPOSAL.md`: EPF7 project proposal draft.
- `docs/BENCHMARK.md`: benchmark commands and required PR metrics.
- `docs/ROADMAP.md`: implementation milestones.

## Data Model

The core state node key is 40 bytes:

```text
8-byte big-endian block number || 32-byte trie node hash
```

This format is represented by `ordereddb.NodeKey`.

Committed data is stored in immutable segment files. A segment covers a bounded
block range, for example:

```text
state-00000000-00004095.sdb
state-00004096-00008191.sdb
state-00008192-00012287.sdb
```

Recent blocks remain in a hot mutable layer until they are safe to seal into a
segment.

Frequently read sealed segments are cached by segment id. The default cache
holds 64 segment readers.

The manifest also records `stateRoot -> blockNumber`, allowing root-based
callers to recover the block context needed by ordered node lookups.

## Compared With LevelDB

LevelDB is a general-purpose LSM-tree. It is write-optimized, but Ethereum state
access needs both high read performance and predictable write behavior during
block synchronization. SolsDB uses Ethereum state data features that LevelDB
does not see:

- State changes are naturally grouped by block.
- State roots provide version boundaries.
- Trie node writes are append-like after block execution.
- Reads can use block/root context to avoid searching LSM levels.

Instead of relying on compaction to reorganize random hash keys, SolsDB seals
state nodes into globally ordered range segments. Segment readers use a block
index plus block-local `nodeHash -> value offset` index, so sealed-state lookups
avoid scanning unrelated levels or unrelated blocks.

## Reported Experimental Results

The following results are from the manuscript
[SolsDB: Solve the Ethereum Bottleneck Cause by Storage](https://potatochipking.github.io/mypaper/SolsDB-Manuscript.pdf).
They were measured with a prototype based on Geth `v1.9.19` and Ethereum
mainnet synchronization workloads. They are not yet results from the current
standalone repository or the planned Geth `v1.17.2` patch.

| Metric | Reported result vs. Geth/LevelDB |
| --- | --- |
| Read performance | up to `4.7x` higher |
| Read QPS | `44.5%` to `475.2%` higher |
| Read latency | `31.1%` to `82.6%` lower |
| Write performance | `25.6%` to `399%` higher |
| Read tail latency | `68.7%` to `83.3%` lower |
| Write latency | `32.4%` to `85.6%` lower |
| Write amplification factor | `49.1%` to `76.1%` lower |

The manuscript attributes the gains to two design choices: using block timeline
features to keep state data globally ordered without LevelDB-style compaction,
and using parser/root context so reads can locate the target segment directly.

## Current Prototype Features

- Immutable range segment files.
- Binary hot journal with magic/version/length/CRC records.
- Manifest persistence and startup recovery.
- Segment checksum validation.
- Root-to-block index.
- Segment reader cache.
- Geth-like adapter surface.
- Unit tests, recovery tests, CI, and local benchmark scaffolding.

## Quick Start

Run tests:

```bash
go test ./...
```

Run local storage benchmarks:

```bash
go test ./... -bench . -benchmem
```

See `docs/BENCHMARK.md` for benchmark details.
