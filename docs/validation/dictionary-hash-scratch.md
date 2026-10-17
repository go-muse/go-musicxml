# Dictionary hash scratch reuse

## Scope

Relative to merged main `3d30b6b7010ed1658bc5c4da672c5948c87fff50`, the private
encoder owns one four-byte count buffer and one 256-byte text buffer per digest
or fingerprint invocation. Previously, each count/text call allocated its own
escaping buffers. The hash writer consumes each slice synchronously under the
`io.Writer` no-retention contract. No global buffer, pool or process cache is used.

The write method, byte encoding, chunk boundaries, work charges, cancellation
checks and fixed logical admission are unchanged. The existing 2,048-byte set
allowance includes encoder/SHA scratch. A size guard checks the relevant fixed
components against its 1,024-byte sub-envelope on each tested architecture;
linux/amd64 measured 640 bytes. This is a logical allowance, not a heap/RSS claim.

## Regression evidence

The test-first allocation guard observed 97 allocations before the fix and
requires at most four for its repeated count/text workload. A separate legacy
reference compares every copied write chunk, full/partial digest, accepted-byte
count, work usage, cancellation poll and error at exact and one-short boundaries.
Cases include empty input, 255/256/257-byte edges, multichunk text, embedded NUL
and a UTF-8 sequence split across the unchanged byte chunk boundary.

All five pinned dictionary digests and fingerprints are checked with independent
concurrent encoders/accounts sharing immutable input rows. Canceled fingerprints
remain zero. Existing caller-copy and certificate ownership tests remain in place.

## Isolated measurements

On 2026-10-17, Go 1.27.2 linux/amd64, GOMAXPROCS=1 and GOMEMLIMIT=384MiB,
five adaptive 300-ms samples per manifest compared the exact baseline codec with
the candidate. Each iteration hashed already verified rows on a fresh default
account and checked the pinned SHA/encoded length. Parsing, sorting and import
setup were outside this measured loop; account/check overhead was included.

| Manifest | Median before ns | Median after ns | Before B/op | After B/op | Before allocs/op | After allocs/op |
|---|---:|---:|---:|---:|---:|---:|
| SMuFL canonical | 1544783 | 539060 | 2300634 | 984 | 20541 | 6 |
| SMuFL ranges | 1646794 | 597893 | 2437398 | 984 | 21720 | 6 |
| Standard Sounds | 315019 | 112167 | 470397 | 984 | 4487 | 6 |
| IPA positive | 128974 | 44328 | 207608 | 984 | 1790 | 6 |
| IPA representation | 302870 | 102217 | 475133 | 984 | 4096 | 6 |

One independent digest of each manifest totals 5,891,170 to 4,920 allocated bytes
and 52,634 to 30 allocations. These are cumulative allocation and isolated digest
runtime observations, not peak live memory, RSS or end-to-end validation speed.
All encoded lengths, hashes and logical work totals matched. Timing is subject to
host noise; the allocation reduction and exact encoding checks are separate proof.

No source/model validation rule, default budget, public loading behavior or
incomplete-assessment interpretation changes. Snapshot sharing and context
memoization are outside this change. Whole-PR acceptance and hosted checks are
recorded in the PR; component evidence alone is not final acceptance.
