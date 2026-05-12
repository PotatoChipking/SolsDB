# Geth Wrapper Example

This example shows the intended composition pattern without importing Geth:

```text
existing Ethereum key/value backend
        +
SolsDB ordered state store
        =
ethadapter.Adapter
```

In a Geth patch, ordinary chain data remains on the existing backend. Trie node
writes should call `PutStateNode(block, hash, rlp)` from the trie commit path
where the block number and state root are known.

Lifecycle shape:

```go
state, _ := ordereddb.OpenFileStore("state-solstdb", ordereddb.Options{})
db := ethadapter.New(existingBackend, state, ethadapter.Config{})

db.BeginBlock(blockNumber, stateRoot)
db.PutStateNode(blockNumber, nodeHash, rlpNode)
db.CommitBlock(blockNumber, stateRoot)
```

The default 40-byte key classifier exists for experiments and tests. Production
Geth integration should use the explicit state-node methods instead.
