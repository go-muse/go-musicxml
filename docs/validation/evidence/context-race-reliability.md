# Race-suite coverage and finite execution headroom

At published head `d9e5fa1`, all nine CI jobs passed. The root race package took
592.369 seconds, leaving less than eight seconds under Go's implicit 600-second
package deadline. The matching local run took 510.972 seconds. These observed
host differences make the remaining CI margin unreliable; a green run alone
is not evidence of adequate timeout headroom.

Independent attribution ran 78 context-named top-level tests (124.309 seconds)
and 86 complementary tests in the context files (77.653 seconds), all passing.
These separate processes identify legitimate broad coverage and repeated setup;
their sum is not a measurement of one complete root-package run.

## Remove repeated work without removing cases

The independent shared-account cost test and a later cleanup test each executed
the same four model assessments: 100 and 1,000 notes, each with 64 MiB and 512 MiB
retention, and the same explicit finite work budget. The second test added depth
and usage-error assertions but no distinct setup, target or completion branch.
Those assertions now run in the existing cost matrix. All four cases, outcome
checks, counters, retention checks and both live-gauge checks remain. The repeated
matrix took 18.02 seconds in the attributed race run and is no longer replayed.

The span-local default matrix has ten properties, five semantic axes and both
source/model adapters: all 100 leaf cases remain. It previously compiled the same
immutable no-timeline engine 50 times. It now compiles once within that top-level
test; every leaf still constructs a fresh account, session, sink, reader/model,
completion and report. No global cache or shared mutable session was introduced.
The same 100 leaf names pass under race. Observed top-level time changed from
15.92 to 1.91 seconds; these measurements are test-setup observations on a shared
host, not production speed guarantees.

## Keep bounded full-suite execution

The local and release race command remains a full uncached `./...` run with the
race detector and `-timeout=15m`. CI now partitions the same package inventory
into root and non-root execution jobs discovered with `go list -race`; each
retains the existing 20-minute cap. The stable `Race detector` aggregate passes
only when both execution jobs succeed. Checked-in contracts protect those finite
limits, the unchanged local command, exact shard coverage, and fail-closed
aggregation. The earlier timeout contract's authentic RED occurred before adding
the explicit package timeout, followed by GREEN after that command change.

No race-only skips, shortened fixtures, removed semantic assertions or reduced
fuzz execution counts are introduced. The duplicate standalone test execution
is consolidated into the retained matrix with all cleanup assertions; existing
cancellation tests are unchanged. Assessment budgets are unchanged. This does not close B5-PERF-01
or claim that general score assessment has become unrestricted. Exact-head local
stages and CI are rerun after this test/CI-only correction.

## Exhaustive CI sharding observation

On the exact pre-sharding tree `5d2264f5ebb3c8e4249d72f84e948fb2437a5fb7`
(local commit `d2a33c69d07ade846aa18426da9d7d449247c6e0`), an unchanged full
race run took 390.254 seconds with four CPUs. Two simultaneous four-CPU shards
took 309.096 seconds for the root and 218.091 seconds for all other packages.
The critical path was about 20.8% shorter, while summed CPU increased roughly
4.9% and summed runner occupancy increased 35.1%. This trades two allocations
for one; it does not claim reduced total compute or predict hosted queue time.
The measured runs used identical seven test-binary hashes, covered all nine
packages exactly once, and preserved all 96,571 terminal test name/action/count
records, including the same three existing skips.

The external measurement used Go 1.27.2 on Linux/amd64 with `GOMAXPROCS=4`,
empty `GOFLAGS`, a warmed build cache, and
`go test -race -count=1 -timeout=15m -p=4 -json`. The baseline supplied `./...`
and ran on CPUs 0–3. The simultaneous shard invocations supplied the root import
path and the other eight discovered import paths respectively, pinned by
`taskset -c 0-3` and `taskset -c 4-7`. A measurement-only `-exec` wrapper recorded
each binary's SHA-256, exit status and child-process usage identically on both
sides. Real xmllint 2.9.14 was available with `MUSICXML_REQUIRE_XMLLINT=1`.
The seven warmed race binaries were hash-identical between these runs. This
experiment partitioned the packages outside the repository script before the
new shard implementation; its timings do not measure that new implementation.
The two allocations shared host RAM, storage, kernel and Go build cache, unlike
two independent hosted runners. Current CI still requires real xmllint in its
separate XSD job; the race jobs do not themselves install it.

This is one matched observation on a shared local host. It excludes hosted
queueing, setup and the aggregate gate, and is not evidence of reliable hosted
sub-ten-minute completion. Exact final-tree local seven-stage checks and hosted
end-to-end acceptance remain pending. Hosted measurement must start at workflow
creation and end at aggregate completion, including both execution jobs and
queue/setup delays. The current workflow expands to eleven checks; earlier
nine-job receipts above remain historical observations for their named heads.
