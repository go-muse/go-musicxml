#!/usr/bin/env bash

# Runs the full set of code checks for this repository.
#
#   bash scripts/check-all.sh              # every stage, in order
#   bash scripts/check-all.sh mod format   # only the named stages
#   bash scripts/check-all.sh short        # the quick run for the edit loop
#
# The checks work on a worktree with uncommitted changes: the mod and
# generate stages compare snapshots of the whole worktree taken before and
# after the command instead of requiring a clean tree.

set -euo pipefail

check_go="${GO:-go}"
check_gofmt="${GOFMT:-gofmt}"
# Fuzz budgets are execution counts, not durations. A count makes negligible
# coverage impossible to conceal behind a nominal time budget, and a duration
# can fail spuriously with "context deadline exceeded" on Go 1.26 because of
# a race in the fuzz coordinator (go.dev/issue/75804, fixed in Go 1.27).
check_fuzz_time="${FUZZ_TIME:-50000x}"
check_mxl_fuzz_time="${MXL_FUZZ_TIME:-10000x}"
check_validation_fuzz_time="${VALIDATION_FUZZ_TIME:-10000x}"

all_stages=(mod generate format test vet race fuzz)
# Stages that run only when named.
named_stages=(short)

stage() {
    echo "==> $*"
}

# tree_hash prints the hash of a tree object built from the whole worktree,
# untracked files included. A temporary index keeps the real index untouched.
tree_hash() {
    local tmp
    tmp="$(mktemp -d)"
    GIT_INDEX_FILE="$tmp/index" git add -A -- .
    GIT_INDEX_FILE="$tmp/index" git write-tree
    rm -rf "$tmp"
}

# run_unchanged runs a command and fails if it changed the worktree.
run_unchanged() {
    local message="$1"
    shift
    local before after
    before="$(tree_hash)"
    "$@"
    after="$(tree_hash)"
    if [[ "$before" != "$after" ]]; then
        echo "$message"
        git diff-tree -r --name-status "$before" "$after"
        exit 1
    fi
}

stage_mod() {
    stage "go mod tidy"
    run_unchanged "go mod tidy changed module files:" "$check_go" mod tidy
}

stage_generate() {
    stage "go generate"
    run_unchanged "go generate changed or created files:" \
        "$check_go" generate ./...
}

stage_format() {
    stage "gofmt"
    local unformatted
    unformatted="$({
        git ls-files -z --cached --others --exclude-standard -- '*.go' |
            while IFS= read -r -d '' path; do
                if [[ -f "$path" ]]; then
                    printf '%s\0' "$path"
                fi
            done |
            xargs -0 "$check_gofmt" -l
    })"
    if [[ -n "$unformatted" ]]; then
        echo "unformatted Go files:"
        echo "$unformatted"
        exit 1
    fi
}

# select_packages fills the caller's `selected` array with one of two disjoint
# sets of packages: the module root, or every other package of ./... . The
# first argument names the stage in messages, the second is root or packages,
# and the rest are passed to `go list`.
select_packages() {
    local label="$1" selection="$2"
    shift 2
    local root inventory status package seen root_count
    selected=()
    # A sentinel preserves trailing newlines so malformed blank records cannot
    # disappear through command substitution. Capture each go-list status
    # directly: partial output from a failed discovery must never start tests.
    if root="$("$check_go" list "$@" .; status=$?; printf '.'; exit "$status")"; then
        root="${root%.}"
        root="${root%$'\n'}"
    else
        return "$?"
    fi
    if [[ -z "$root" || "$root" == -* || "$root" == *[[:space:][:cntrl:]]* ]]; then
        echo "$label discovery requires one nonempty root import path" >&2
        return 2
    fi
    if inventory="$("$check_go" list "$@" ./...; status=$?; printf '.'; exit "$status")"; then
        inventory="${inventory%.}"
        inventory="${inventory%$'\n'}"
    else
        return "$?"
    fi
    seen=$'\n'
    root_count=0
    while IFS= read -r package; do
        if [[ -z "$package" || "$package" == -* || "$package" == *[[:space:][:cntrl:]]* ]]; then
            echo "$label discovery returned an empty or malformed import path" >&2
            return 2
        fi
        if [[ "$seen" == *$'\n'"$package"$'\n'* ]]; then
            echo "$label discovery returned a duplicate import path: $package" >&2
            return 2
        fi
        seen+="$package"$'\n'
        if [[ "$package" == "$root" ]]; then
            root_count=$((root_count + 1))
            if [[ "$selection" == root ]]; then
                selected+=("$package")
            fi
        elif [[ "$selection" == packages ]]; then
            selected+=("$package")
        fi
    done <<< "$inventory"
    if [[ "$root_count" != 1 || "${#selected[@]}" == 0 ]]; then
        echo "$label discovery requires exactly one root and a nonempty selected shard" >&2
        return 2
    fi
}

# CI can split the test stage into two disjoint shards. Shard 1 runs the root
# package's tests whose names match this pattern; shard 2 runs the root
# package with the same pattern skipped, then every other package. A root test
# either matches the pattern or does not, and examples and fuzz seeds never
# match it, so each of them runs in exactly one shard. The pattern only
# balances the two shards' times on the hosted runner.
check_test_shard_pattern='^Test[A-F]'

stage_test() {
    stage "go test"
    # Local and release checks retain the original exhaustive command.
    if [[ ! ${TEST_SHARD+x} ]]; then
        "$check_go" test -count=1 ./...
        return
    fi
    case "$TEST_SHARD" in
        1|2) ;;
        *) echo "invalid TEST_SHARD: expected 1 or 2" >&2; return 2 ;;
    esac
    if [[ "$TEST_SHARD" == 1 ]]; then
        "$check_go" test -count=1 -run="$check_test_shard_pattern" .
        return
    fi
    local selected=()
    select_packages test packages
    "$check_go" test -count=1 -skip="$check_test_shard_pattern" .
    "$check_go" test -count=1 "${selected[@]}"
}

# stage_short is the edit loop's run: every package with -short, which leaves
# out the tests that take a second or more. It is not part of the default
# sequence, and it replaces no stage of it.
stage_short() {
    stage "go test -short"
    "$check_go" test -short -count=1 ./...
}

stage_vet() {
    stage "go vet"
    "$check_go" vet ./...
}

stage_race() {
    stage "go test -race"
    # Local and release checks retain the original exhaustive command. CI can
    # opt into exactly one of two disjoint, dynamically discovered shards.
    if [[ ! ${RACE_SHARD+x} ]]; then
        "$check_go" test -race -count=1 -timeout=15m ./...
        return
    fi
    case "$RACE_SHARD" in
        root|packages) ;;
        *) echo "invalid RACE_SHARD: expected root or packages" >&2; return 2 ;;
    esac

    local selected=()
    select_packages race "$RACE_SHARD" -race
    "$check_go" test -race -count=1 -timeout=15m "${selected[@]}"
}

stage_fuzz() {
    stage "fuzz"
    "$check_go" test -run='^$' -fuzz='^FuzzDocumentRoundTrip$' \
        -fuzztime="$check_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzMXLPackageRoundTrip$' \
        -fuzztime="$check_mxl_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzMXLAssessmentZIPObserver$' \
        -fuzztime="$check_mxl_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzMXLLinkResolution$' \
        -fuzztime="$check_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzSourceXMLReader$' \
        -fuzztime="$check_validation_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzSourceInternalDTDFrames$' \
        -fuzztime="$check_validation_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzAssessCurrentModel$' \
        -fuzztime="$check_validation_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzContextAssessment$' \
        -fuzztime="$check_validation_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzAuxiliarySourceAssessment$' \
        -fuzztime="$check_validation_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzReportFieldIsBoundedEscapedUTF8$' \
          -fuzztime="$check_validation_fuzz_time" ./validation
    "$check_go" test -run='^$' -fuzz='^FuzzSessionLifecycle$' \
          -fuzztime="$check_validation_fuzz_time" ./validation
}

if [[ $# -eq 0 ]]; then
    set -- "${all_stages[@]}"
fi

for name in "$@"; do
    case " ${all_stages[*]} ${named_stages[*]} " in
        *" $name "*) "stage_$name" ;;
        *)
            echo "unknown stage: $name" >&2
            echo "stages: ${all_stages[*]} ${named_stages[*]}" >&2
            exit 2
            ;;
    esac
done
