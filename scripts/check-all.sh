#!/usr/bin/env bash

# Runs the full set of code checks for this repository.
#
#   bash scripts/check-all.sh              # every stage, in order
#   bash scripts/check-all.sh mod format   # only the named stages
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

all_stages=(mod generate format test vet race fuzz)

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

stage_test() {
    stage "go test"
    "$check_go" test -count=1 ./...
}

stage_vet() {
    stage "go vet"
    "$check_go" vet ./...
}

stage_race() {
    stage "go test -race"
    "$check_go" test -race -count=1 ./...
}

stage_fuzz() {
    stage "fuzz"
    "$check_go" test -run='^$' -fuzz='^FuzzDocumentRoundTrip$' \
        -fuzztime="$check_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzMXLPackageRoundTrip$' \
        -fuzztime="$check_mxl_fuzz_time" .
    "$check_go" test -run='^$' -fuzz='^FuzzMXLLinkResolution$' \
        -fuzztime="$check_fuzz_time" .
}

if [[ $# -eq 0 ]]; then
    set -- "${all_stages[@]}"
fi

for name in "$@"; do
    case " ${all_stages[*]} " in
        *" $name "*) "stage_$name" ;;
        *)
            echo "unknown stage: $name" >&2
            echo "stages: ${all_stages[*]}" >&2
            exit 2
            ;;
    esac
done
