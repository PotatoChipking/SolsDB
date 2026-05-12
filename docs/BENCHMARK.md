# Benchmark Guide

SolsDB includes package-level benchmarks for the local storage and adapter
paths. These are not a replacement for Geth import benchmarks, but they give a
stable baseline for storage-level changes.

Run all local benchmarks:

```bash
go test ./... -bench . -benchmem
```

Useful focused runs:

```bash
go test ./ordereddb -bench 'FileStore|MemStore' -benchmem
go test ./ethadapter -bench 'Adapter' -benchmem
```

Current benchmark groups:

- hot in-memory state node reads
- hot file-store state node reads
- sealed segment state node reads
- adapter explicit state node reads
- adapter ordinary key/value reads

For Geth PR material, supplement these with client-level measurements:

- block import time
- state commit latency
- trie node read latency
- duplicate node rate
- disk bytes written
- segment count
- restart recovery time
- segment cache hit ratio
