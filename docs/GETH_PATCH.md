# Geth Patch Plan

This document describes the intended Geth-side patch needed to evaluate SolsDB
as an ordered state trie node backend.

The goal is a small, reviewable integration patch. Ordinary chain data should
continue to use Geth's existing database backend. Only state trie node payloads
should be routed into SolsDB.

This plan is currently pinned to go-ethereum `v1.17.2`. See
`docs/GETH_BASELINE.md` for checkout and toolchain details.

## Patch Strategy

Use SolsDB as a sidecar state backend:

```text
headers / bodies / receipts / metadata -> existing ethdb backend
state trie node RLPs                  -> SolsDB ordereddb.Store
```

The Geth patch should pass explicit block context to SolsDB. It should not rely
on encoding block numbers into Geth's ordinary key/value keys.

## New Wiring

Add configuration for the experimental backend:

```text
--state.backend=leveldb|solsdb
--solsdb.path=<path>
--solsdb.segment-blocks=4096
--solsdb.segment-bytes=268435456
--solsdb.hot-window=256
```

The exact flag names can change to fit Geth conventions.

At database open time:

1. Open the normal chain database as usual.
2. If SolsDB is enabled, open `ordereddb.OpenFileStore`.
3. Wrap both backends with `ethadapter.Adapter`.
4. Pass the wrapped adapter into the state/trie paths that need node storage.

## Required Hooks

### Block Import Start

Before state execution for a block:

```go
adapter.BeginBlock(block.NumberU64(), stateRoot)
```

The initial root may be the parent root if the final root is not known yet.

### Trie Node Commit

During trie database commit, route each node write:

```go
adapter.PutStateNode(blockNumber, nodeHash, rlpNode)
```

This is the critical hook. Geth normally stores trie nodes by `hash -> rlp`.
SolsDB needs `blockNumber + hash -> rlp`, so the block number must be available
in the commit context.

### Block Import Success

After the block is fully imported and the post-state root is known:

```go
adapter.CommitBlock(block.NumberU64(), postStateRoot)
```

### Block Import Failure

If execution/import fails:

```go
adapter.AbortBlock(block.NumberU64())
```

### Canonical Chain Update

When a block becomes canonical:

```go
adapter.Canonicalize(block.NumberU64(), block.Root())
```

### Reorg

When the canonical chain rolls back:

```go
adapter.RollbackTo(commonAncestor.NumberU64())
```

### Pruning

When old state can be pruned:

```go
adapter.PruneBefore(retainFromBlock)
```

## Candidate Geth Areas

The exact files depend on the Geth version. Look for these areas:

- database open/configuration code
- block import processor
- state processor
- trie database commit path
- chain canonicalization and reorg logic
- state pruning logic

The trie commit path is the main design risk because it must receive block
context without contaminating unrelated storage writes.

## Integration Modes

### Explicit Hook Mode

Preferred mode. Geth calls SolsDB's explicit state-node API:

```go
PutStateNode(block, hash, rlp)
```

This avoids changing Geth's ordinary key format.

### Classifier Mode

Development-only mode. A 40-byte key is classified as:

```text
8-byte block number || 32-byte hash
```

This mode is useful for tests and experiments but should not be the production
Geth integration path.

## Test Plan

Minimum Geth-side tests:

- block import writes state nodes to SolsDB
- ordinary chain data remains in the existing backend
- failed block import aborts pending SolsDB writes
- restart recovers committed hot blocks
- sealed segments survive restart
- reorg removes noncanonical hot blocks
- pruning marks old segments prunable
- missing node errors are compatible with trie sync/healing behavior

## Benchmark Plan

Compare default Geth backend and SolsDB sidecar backend:

- block import time
- trie node read latency
- state commit latency
- disk bytes written
- segment count
- restart recovery time
- cache hit ratio once a segment cache exists

## Review Notes

Keep the first Geth patch narrow:

- no consensus logic changes
- no chain data format changes
- no ancient/freezer changes
- no changes to receipt/header/body storage
- feature flag disabled by default

The first reviewable target is an experimental backend that can import blocks
and demonstrate measurable state trie storage behavior.
