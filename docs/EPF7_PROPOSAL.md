# EPF7 Project Proposal: SolsDB

## Title

SolsDB: Block-aware state trie storage prototype for Ethereum execution clients

## Summary

SolsDB explores a block-aware ordered storage engine for Ethereum state trie
nodes. The project aims to reduce read/write amplification in execution clients
by storing trie nodes in block-range segments instead of relying only on a
general-purpose LSM key/value layout.

The EPF7 goal is to build and evaluate a minimal Geth `v1.17.2` prototype that
routes Ethereum state trie node writes into SolsDB while preserving ordinary
chain data on the existing backend.

## Motivation

Ethereum execution clients store large amounts of state trie data. In the
standard key/value layout, trie nodes are commonly addressed by content hash:

```text
nodeHash -> rlpNode
```

This layout is simple and content-addressed, but it hides useful timeline
information from the storage engine. State nodes from different blocks are mixed
by random-looking hashes, causing reads to search unrelated LSM levels and
writes to interact with compaction.

SolsDB adds explicit block context:

```text
blockNumber + nodeHash -> rlpNode
```

This makes it possible to organize state trie data in block order, seal recent
state changes into immutable block-range segments, and use state root/block
metadata to locate data more directly.

## Fit With EPF7

This project fits the EPF scope as execution-client research and prototyping:

- execution client storage optimization
- state trie access and persistence
- client benchmarking
- correctness testing around state roots, restart, reorg, and pruning

The project is not an application-layer or smart-contract project. It targets
Ethereum protocol/client infrastructure.

## Current State

The standalone SolsDB repository already contains:

- immutable range segment files
- binary hot journal with magic/version/length/CRC records
- manifest persistence and startup recovery
- segment checksum validation
- `stateRoot -> blockNumber` root index
- segment reader cache
- block-local `nodeHash -> value offset` segment index
- Geth-like `ethadapter` package
- recovery tests and local benchmark scaffolding
- Geth integration planning documents

The current code is a standalone prototype. It is not yet integrated into Geth.

## EPF7 Goal

Build and evaluate a minimal Geth `v1.17.2` integration prototype for SolsDB.

The prototype should prove:

1. Geth can pass block context into the state trie node commit path.
2. SolsDB can receive state node writes through:

   ```go
   PutStateNode(blockNumber, nodeHash, rlpNode)
   ```

3. Ordinary chain data remains on the existing Geth backend.
4. Restart, abort, reorg, and state-root correctness can be tested.
5. Storage-level and client-level benchmarks can be collected.

## Proposed Milestones

### Month 1: Geth Integration Study

- Pin target baseline to go-ethereum `v1.17.2`.
- Identify Geth call paths for:
  - database open/configuration
  - block import
  - state processing
  - trie database commit
  - canonical chain updates
  - reorg and pruning
- Produce a minimal patch plan showing where block number and state root context
  can be passed into trie node writes.

Deliverables:

- Geth patch notes
- call graph notes
- updated `GETH_PATCH.md`

### Month 2: Minimal Geth PoC

- Add feature flag/config for SolsDB sidecar state backend.
- Open normal Geth backend for ordinary chain data.
- Open SolsDB for state trie node payloads.
- Hook:
  - `BeginBlock`
  - `PutStateNode`
  - `CommitBlock`
  - `AbortBlock`

Deliverables:

- Geth branch or patch set based on `v1.17.2`
- minimal block import path using SolsDB for state nodes

### Month 3: Correctness

- Verify state root consistency against the default backend.
- Test restart recovery.
- Test failed block import / abort.
- Test reorg behavior inside the hot window.
- Analyze missing node error compatibility with Geth trie sync/healing.

Deliverables:

- correctness test report
- failing edge cases documented
- required changes for missing-node compatibility

### Month 4: Performance Evaluation

- Measure local SolsDB benchmarks:
  - hot reads
  - sealed segment reads
  - adapter overhead
- Measure Geth-level behavior:
  - block import time
  - state commit latency
  - trie node read latency
  - disk bytes written
  - segment count
  - duplicate node rate
  - restart recovery time
  - segment cache hit/miss

Deliverables:

- benchmark scripts
- benchmark results
- comparison with default Geth backend

### Month 5: Report and PR-ready Artifact

- Produce final design report.
- Document limitations:
  - archive mode
  - pruning safety
  - deep reorg behavior
  - duplicate trie node storage
- Prepare a reviewable Geth patch or draft PR.
- Open an upstream discussion if the prototype results justify it.

Deliverables:

- final technical report
- Geth patch branch or draft PR
- benchmark report
- future work list

## Success Criteria

The project is successful if it produces:

- a working Geth `v1.17.2` prototype
- correctness evidence for state root preservation
- restart and abort recovery tests
- benchmark data comparing SolsDB with the default backend
- a clear explanation of tradeoffs and limitations

Upstream merge is not required for success. The expected EPF7 output is a
research-quality client prototype and evaluation.

## Risks and Mitigations

### Risk: Block context is hard to pass into trie commit

Mitigation: keep the first patch narrow and explicitly trace the state commit
call path before implementing broader storage changes.

### Risk: Duplicate trie nodes increase disk usage

Mitigation: measure duplicate node rate first. Add segment-local or global
deduplication only after the baseline prototype is correct.

### Risk: Pruning can delete still-referenced nodes

Mitigation: disable pruning by default in the first Geth prototype. Treat safe
pruning as follow-up work requiring retained-root reachability analysis.

### Risk: Deep reorg exceeds hot window

Mitigation: document behavior and start with conservative hot-window settings.
Prefer sealing finalized blocks when finality information is available.

### Risk: Performance gains do not reproduce on latest Geth

Mitigation: report negative or mixed results honestly. The project still
contributes useful data about Ethereum state storage design.

## Prior Work

The SolsDB manuscript reports prototype results based on Geth `v1.9.19` and
Ethereum mainnet synchronization workloads:

- read performance up to `4.7x` higher
- read QPS `44.5%` to `475.2%` higher
- read latency `31.1%` to `82.6%` lower
- write performance `25.6%` to `399%` higher
- read tail latency `68.7%` to `83.3%` lower
- write latency `32.4%` to `85.6%` lower
- write amplification factor `49.1%` to `76.1%` lower

Reference:

https://potatochipking.github.io/mypaper/SolsDB-Manuscript.pdf

These results motivate the EPF7 project but should be re-evaluated on the new
Geth `v1.17.2` prototype.

## References

- EPF7 announcement: https://blog.ethereum.org/2026/04/30/epf-7
- Protocol Fellowship: https://ps.ethereum.foundation/fellowship
- EPF7 program details: https://github.com/eth-protocol-fellows/cohort-seven/blob/main/program-guide/program-details.md
- Geth baseline: `docs/GETH_BASELINE.md`
- SolsDB design: `docs/DESIGN.md`
- Geth patch plan: `docs/GETH_PATCH.md`

## Application Form Draft Answers

Use these answers as a concise application draft. Adjust personal background,
availability, and contact details before submitting.

### Project Title

SolsDB: Block-aware state trie storage prototype for Ethereum execution clients

### One-line Summary

I want to build and evaluate a Geth `v1.17.2` prototype that stores Ethereum
state trie nodes in block-aware ordered segments to reduce state storage
read/write amplification.

### Project Area

Execution client implementation, state storage, benchmarking, and protocol
infrastructure research.

### Problem Statement

Ethereum execution clients store large volumes of state trie node data. In the
standard key/value layout, trie nodes are usually addressed by content hash
(`nodeHash -> rlpNode`). This is simple, but it hides block timeline information
from the storage engine. Random-looking trie node hashes mix state from many
blocks across LSM levels, which can increase read amplification, write
amplification, and compaction pressure during synchronization and state access.

SolsDB explores whether making state trie storage block-aware can improve this
behavior. The core idea is to store state nodes as `blockNumber + nodeHash ->
rlpNode`, seal committed state into immutable block-range segments, and use
state root metadata to recover block context for reads.

### Proposed Work

During EPF7, I will build a minimal Geth `v1.17.2` prototype that routes state
trie node writes into SolsDB while preserving ordinary chain data on Geth's
existing backend.

The work will include:

1. Tracing Geth's block import and trie commit paths.
2. Passing block number and state root context into trie node commits.
3. Calling SolsDB through explicit methods such as:

   ```go
   PutStateNode(blockNumber, nodeHash, rlpNode)
   ```

4. Preserving headers, bodies, receipts, tx lookup data, freezer data, and
   metadata on the normal backend.
5. Testing state-root correctness, restart recovery, abort handling, and reorg
   behavior.
6. Benchmarking SolsDB against the default Geth backend.

### Why This Matters for Ethereum

State growth and state access remain important execution-client concerns.
Storage layout affects sync time, disk usage, read latency, and write
amplification. A block-aware state storage prototype can provide useful data for
client teams, even if the final design is not immediately upstreamed.

The expected output is not only code, but also evidence: correctness tests,
benchmark results, design tradeoffs, and a reviewable Geth patch.

### Current Progress

The standalone SolsDB repository already includes:

- immutable block-range segment files
- binary hot journal with magic/version/length/CRC records
- manifest persistence and startup recovery
- segment checksum validation
- `stateRoot -> blockNumber` root index
- block-local `nodeHash -> value offset` segment index
- segment reader cache
- Geth-like adapter package
- recovery tests and benchmark scaffolding
- Geth patch planning documents

The project is ready for the next step: a real Geth `v1.17.2` integration
prototype.

### Milestones

Month 1:

- Study Geth `v1.17.2` block import, state processing, and trie commit paths.
- Produce a precise patch plan and call graph.

Month 2:

- Implement a minimal Geth feature-flagged SolsDB sidecar backend.
- Route state node writes through `PutStateNode`.

Month 3:

- Add correctness tests for state root consistency, restart recovery, aborts,
  and reorgs.
- Analyze missing-node error compatibility.

Month 4:

- Run storage-level and Geth-level benchmarks.
- Measure import time, state commit latency, trie reads, disk usage, duplicate
  node rate, and recovery time.

Month 5:

- Prepare final report, benchmark results, limitations, and a reviewable Geth
  patch or draft PR.

### Expected Deliverables

- Geth `v1.17.2` patch branch or draft PR.
- SolsDB integration notes and call graph.
- Correctness test report.
- Benchmark report comparing SolsDB with the default backend.
- Documentation of limitations around pruning, deep reorgs, archive mode, and
  deduplication.

### Risks

The main risk is that Geth's trie commit path may not pass block context cleanly.
I will mitigate this by first producing a narrow call-path study and limiting
the first patch to state trie node writes only.

Another risk is disk amplification from storing `blockNumber + nodeHash` when
the same trie node is reused across blocks. I will measure duplicate node rate
before implementing deduplication.

Pruning safety is also risky because trie nodes can be shared across roots. The
first prototype will keep pruning conservative or disabled by default.

### Why I Am a Good Fit

I already have a working standalone prototype and prior research results for
SolsDB. I understand the storage-engine side of the problem and want to use EPF7
to turn the prototype into a realistic execution-client experiment with
correctness tests and benchmark evidence.

### Scope Boundary

This project will not attempt to replace Geth's full database stack. It will
only target state trie node payloads. Ordinary chain data and freezer/ancient
data stay on the default backend.

The success target is a research-quality prototype and evaluation, not immediate
upstream merge.
