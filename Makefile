.PHONY: dist test clean all

VERSION := $(shell git describe --always)
GO_BUILD := go build -ldflags "-X main.version=$(VERSION)"

DIR_BIN := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))/bin

DIST = dist/%
DISTS := $(patsubst cmd/%,$(DIST),$(wildcard cmd/*))

TARGETS = $(DISTS)

SRCS_OTHER := $(shell find . \
	-type d -name cmd -prune -o \
	-type f -name "*.go" -print) go.mod

all: $(TARGETS)
	@echo "$@ done." 1>&2

clean:
	/bin/rm -f $(TARGETS)
	@echo "$@ done." 1>&2

TOOL_STATICCHECK = $(DIR_BIN)/staticcheck
TOOLS = \
	$(TOOL_STATICCHECK)

TOOLS_DEP = Makefile

.PHONY: tools
tools: $(TOOLS)
	@echo "$@ done." 1>&2

.PHONY: dist
dist: $(DISTS)
	@echo "$@ done." 1>&2

.PHONY: lint
lint: $(TOOL_STATICCHECK)
	$(TOOL_STATICCHECK) ./...

$(TOOL_STATICCHECK): export GOBIN=$(DIR_BIN)
$(TOOL_STATICCHECK): $(TOOLS_DEP)
	@echo "### `basename $@` install destination=$(GOBIN)" 1>&2
	go install honnef.co/go/tools/cmd/staticcheck@v0.4.7

$(DIST): ./cmd/%/* $(SRC_OTHER)
	$(GO_BUILD) -o $@ ./cmd/$*/
