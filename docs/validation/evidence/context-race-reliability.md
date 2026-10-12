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

The shared local/CI race command remains a full uncached `./...` run with the race
detector. It now explicitly uses `-timeout=15m`; the CI job retains its existing
20-minute cap. A checked-in contract test protects those finite limits and the
full race command. Its authentic RED occurred before adding the explicit package
timeout, followed by GREEN after the command change.

No race-only skips, shortened fixtures, removed semantic assertions or reduced
fuzz execution counts are introduced. The duplicate standalone test execution
is consolidated into the retained matrix with all cleanup assertions; existing
cancellation tests are unchanged. Assessment budgets are unchanged. This does not close B5-PERF-01
or claim that general score assessment has become unrestricted. Exact-head local
stages and CI are rerun after this test/CI-only correction.
