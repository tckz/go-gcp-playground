.PHONY: dist test clean all

VERSION := $(shell git describe --always)
GO_BUILD := go build -ldflags "-X main.version=$(VERSION)"

DIR_BIN := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))/bin
DIR_DIST = dist

DISTS = \
	$(DIR_DIST)/datastore-delete-all \
	$(DIR_DIST)/datastore-get \
    $(DIR_DIST)/datastore-load-random \
    $(DIR_DIST)/datastore-put \
    $(DIR_DIST)/datastore-putex \
    $(DIR_DIST)/datastore-query \
    $(DIR_DIST)/issue-id-token \
    $(DIR_DIST)/issue-id-token2 \
    $(DIR_DIST)/pubsub-publish-random \
    $(DIR_DIST)/pubsub-subscriber \
    $(DIR_DIST)/pubsub-subscriber-dump \
    $(DIR_DIST)/sign-jwt

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
	go install honnef.co/go/tools/cmd/staticcheck@v0.4.3

$(DIR_DIST)/datastore-delete-all: cmd/datastore-delete-all/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/datastore-delete-all/

$(DIR_DIST)/datastore-get: cmd/datastore-get/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/datastore-get/

$(DIR_DIST)/datastore-load-random: cmd/datastore-load-random/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/datastore-load-random/

$(DIR_DIST)/datastore-put: cmd/datastore-put/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/datastore-put/

$(DIR_DIST)/datastore-putex: cmd/datastore-putex/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/datastore-putex/

$(DIR_DIST)/datastore-query: cmd/datastore-query/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/datastore-query/

$(DIR_DIST)/issue-id-token: cmd/issue-id-token/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/issue-id-token/

$(DIR_DIST)/issue-id-token2: cmd/issue-id-token2/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/issue-id-token2/

$(DIR_DIST)/pubsub-publish-random: cmd/pubsub-publish-random/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/pubsub-publish-random/

$(DIR_DIST)/pubsub-subscriber: cmd/pubsub-subscriber/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/pubsub-subscriber/

$(DIR_DIST)/pubsub-subscriber-dump: cmd/pubsub-subscriber-dump/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/pubsub-subscriber-dump/

$(DIR_DIST)/sign-jwt: cmd/sign-jwt/* $(SRCS_OTHER)
	$(GO_BUILD) -o $@ ./cmd/sign-jwt/
