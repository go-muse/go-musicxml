#!/usr/bin/env bash

set -euo pipefail

check_go="${GO:-go}"
check_gofmt="${GOFMT:-gofmt}"
check_fuzz_time="${FUZZ_TIME:-10s}"
check_mxl_fuzz_time="${MXL_FUZZ_TIME:-10000x}"

"$check_go" mod tidy
module_changes="$(git status --porcelain -- go.mod go.sum)"
if [[ -n "$module_changes" ]]; then
    echo "go mod tidy changed module files:"
    echo "$module_changes"
    exit 1
fi

"$check_go" generate ./...
generated_changes="$(git status --porcelain --untracked-files=all)"
if [[ -n "$generated_changes" ]]; then
    echo "go generate changed or created files:"
    echo "$generated_changes"
    exit 1
fi

unformatted="$({
    git ls-files -z --cached --others --exclude-standard -- '*.go' |
        xargs -0 "$check_gofmt" -l
})"
if [[ -n "$unformatted" ]]; then
    echo "unformatted Go files:"
    echo "$unformatted"
    exit 1
fi

"$check_go" test -count=1 ./...
"$check_go" vet ./...
"$check_go" test -race -count=1 ./...
"$check_go" test -run='^$' -fuzz='^FuzzDocumentRoundTrip$' \
    -fuzztime="$check_fuzz_time" .
"$check_go" test -run='^$' -fuzz='^FuzzMXLPackageRoundTrip$' \
    -fuzztime="$check_mxl_fuzz_time" .
"$check_go" test -run='^$' -fuzz='^FuzzMXLLinkResolution$' \
    -fuzztime="$check_fuzz_time" .
