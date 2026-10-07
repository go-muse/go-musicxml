GO ?= go
GOFMT ?= gofmt
FUZZ_TIME ?= 50000x
MXL_FUZZ_TIME ?= 10000x

CHECK_ALL = GO='$(GO)' GOFMT='$(GOFMT)' FUZZ_TIME='$(FUZZ_TIME)' \
	MXL_FUZZ_TIME='$(MXL_FUZZ_TIME)' bash scripts/check-all.sh

.PHONY: check check-all format format-check fuzz generate generated mod-check test vet race

check: format-check test vet

format:
	find . -name '*.go' -type f -print0 | xargs -0 $(GOFMT) -w

format-check:
	$(CHECK_ALL) format

generate:
	$(GO) generate ./...

generated:
	$(CHECK_ALL) generate

mod-check:
	$(CHECK_ALL) mod

test:
	$(CHECK_ALL) test

vet:
	$(CHECK_ALL) vet

race:
	$(CHECK_ALL) race

fuzz:
	$(CHECK_ALL) fuzz

check-all:
	$(CHECK_ALL)
