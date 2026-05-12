# Roadmap

## Phase 1: Project Boundary

- Define stable key encoding and public storage interfaces.
- Document the storage model and Ethereum integration plan.
- Keep the project independent from earlier experimental code.

## Phase 2: Durable Block Store

- Implement block writer buffers.
- Implement immutable segment files.
- Implement manifest and binary hot block commit journal.
- Implement startup recovery.
- Add focused unit tests for key encoding, manifest recovery, and block reads.

## Phase 3: Ethereum Adapter

- Add a Geth-compatible adapter package.
- Route trie node writes through `BlockWriter`.
- Preserve ordinary chain data on the standard backend.
- Add reorg, pruning, and missing-node behavior.
- Next external milestone: build a minimal patch against a pinned Geth commit.

## Phase 4: Benchmarks and PR Material

- Maintain package-level benchmark scaffolding for hot reads, sealed segment
  reads, and adapter reads.
- Run CI with formatting, unit tests, and race tests.
- Benchmark against Geth's default LevelDB path.
- Measure import latency, trie read latency, write amplification, disk usage,
  and recovery time.
- Publish design limits and integration patch instructions.
