# Ethereum Integration

## Integration Strategy

The first integration target is a Geth-compatible adapter, not a full node fork.
The adapter should keep ordinary chain data on the existing key/value backend
and route state trie node writes through SolsDB.

The pinned Geth baseline is go-ethereum `v1.17.2`. See
`docs/GETH_BASELINE.md` before starting the client patch.

## Required Adapter Surface

The adapter must provide:

- standard key/value methods used by `ethdb`
- batched writes
- trie node read/write interception
- block lifecycle hooks
- close and recovery behavior

The repository provides a dependency-light `ethadapter` package for this
boundary. It intentionally mirrors the small subset of `ethdb` used by storage
callers without importing Geth directly.

```text
ordinary key/value data -> wrapped Ethereum backend
state node data         -> ordereddb.Store
```

The default classifier recognizes SolsDB's 40-byte ordered node key:

```text
8-byte block number || 32-byte node hash
```

For a production Geth patch, this classifier should be replaced or bypassed by
the trie commit path because Geth's native trie node key is normally just the
32-byte node hash. The block context must come from the block import/state commit
pipeline, not from ordinary chain data writes.

The adapter currently supports:

- `Has`
- `Get`
- `Put`
- `Delete`
- `NewBatch`
- `NewIterator`
- `Stat`
- `Compact`
- `Close`
- explicit state-node methods
- block lifecycle methods

See `examples/geth-wrapper` for the dependency-light composition pattern.

See `docs/GETH_PATCH.md` for the proposed Geth-side patch plan.

## Geth Touch Points

Expected integration points:

- trie database commit path
- state root and block number propagation
- missing node handling
- chain canonicalization and reorg handling
- pruning lifecycle

Minimal expected patch shape:

1. Open the existing chain database as the ordinary backend.
2. Open SolsDB as the state node backend.
3. Wrap both with `ethadapter.Adapter`.
4. At block processing start, call `BeginBlock(blockNumber, stateRoot)`.
5. During trie commit, route node writes to `PutNode(blockNumber, hash, rlp)`.
6. After a successful block import, call `CommitBlock(blockNumber, stateRoot)`.
7. On import failure, call `AbortBlock(blockNumber)`.
8. On canonical updates, call `Canonicalize`.
9. On reorg and pruning, call `RollbackTo` and `PruneBefore`.

The preferred implementation is a wrapper around Geth storage interfaces so the
client patch remains small and reviewable.

## Compatibility Rules

- Headers, bodies, receipts, tx lookup data, freezer data, and metadata remain
  on the normal Ethereum database path.
- Only state trie node payloads are routed to SolsDB.
- Missing nodes must be reported with errors compatible with Ethereum trie
  healing and sync paths.
- Archive and pruned modes must be declared explicitly.

## Validation

Before proposing this as a PR reference, the adapter needs:

- Ethereum trie tests
- state transition tests
- reorg tests
- interrupted block import recovery tests
- benchmark results against the default Geth backend
