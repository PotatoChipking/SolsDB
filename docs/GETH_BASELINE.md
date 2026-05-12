# Geth Baseline

The current SolsDB Geth integration target is:

```text
go-ethereum v1.17.2
release: EMF Suppressor
tag: v1.17.2
commit: be4dc0c
release date: 2026-03-30
```

Reference sources:

- https://github.com/ethereum/go-ethereum/releases
- https://geth.ethereum.org/downloads

## Checkout

```bash
git clone https://github.com/ethereum/go-ethereum.git
cd go-ethereum
git checkout v1.17.2
```

## Toolchain

Geth v1.17.2 release notes specify Go 1.25.7 for official builds. A local
integration workspace should use Go 1.25.x before attempting to compile the
Geth patch.

The current SolsDB module itself remains on Go 1.21 so the standalone prototype
can be developed on older local toolchains. The Geth integration branch may
need a newer Go toolchain because it builds go-ethereum.

## Patch Policy

The first Geth integration patch should be based on this tag and should remain
feature-flagged and disabled by default. Do not target moving `master` until the
minimal patch works on the pinned release.
