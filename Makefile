GO ?= go
GOFMT ?= gofmt
FUZZ_TIME ?= 10s
MXL_FUZZ_TIME ?= 10000x

.PHONY: check format format-check fuzz generate generated mod-check release-check test vet race

check: format-check test vet

format:
	find . -name '*.go' -type f -print0 | xargs -0 $(GOFMT) -w

format-check:
	@unformatted="$$(find . -name '*.go' -type f -print0 | xargs -0 $(GOFMT) -l)"; \
	if [ -n "$$unformatted" ]; then \
		echo "unformatted Go files:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

generate:
	$(GO) generate ./...

generated: generate
	@changes="$$(git status --porcelain --untracked-files=all)"; \
	if [ -n "$$changes" ]; then \
		echo "go generate changed or created files:"; \
		echo "$$changes"; \
		exit 1; \
	fi

mod-check:
	$(GO) mod tidy
	@changes="$$(git status --porcelain --untracked-files=all -- go.mod go.sum)"; \
	if [ -n "$$changes" ]; then \
		echo "go mod tidy changed module files:"; \
		echo "$$changes"; \
		exit 1; \
	fi

test:
	$(GO) test -count=1 ./...

vet:
	$(GO) vet ./...

race:
	$(GO) test -race -count=1 ./...

fuzz:
	$(GO) test -run='^$$' -fuzz='^FuzzDocumentRoundTrip$$' -fuzztime=$(FUZZ_TIME) .
	$(GO) test -run='^$$' -fuzz='^FuzzMXLPackageRoundTrip$$' -fuzztime=$(MXL_FUZZ_TIME) .
	$(GO) test -run='^$$' -fuzz='^FuzzMXLLinkResolution$$' -fuzztime=$(FUZZ_TIME) .

release-check:
	GO='$(GO)' GOFMT='$(GOFMT)' FUZZ_TIME='$(FUZZ_TIME)' \
		MXL_FUZZ_TIME='$(MXL_FUZZ_TIME)' bash scripts/release-check.sh
