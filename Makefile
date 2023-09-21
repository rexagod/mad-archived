# Variables are declared in the order in which they occur.
VALE_VERSION ?= 2.28.0
ASSETS_DIR ?=
GO ?= go
MARKDOWNFMT_VERSION ?= v3.1.0
GOLANGCI_LINT_VERSION ?= v1.54.2
ifeq ($(ASSETS_DIR),)
    MD_FILES = $(shell find . \( -type d -name '.vale' \) -prune -o -type f -name "*.md" -print)
else
    MD_FILES = $(shell find . \( -type d -name '.vale' -o -type d -name $(patsubst %/,%,$(patsubst ./%,%,$(ASSETS_DIR))) \) -prune -o -type f -name "*.md" -print)
endif
GO_FILES = $(shell find . -type f -name "*.go")
OS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH ?= $(shell $(GO) env GOARCH)
E2E_TEST_PKG = github.com/rexagod/mad/tests
COMMON = github.com/prometheus/common
VERSION = $(shell cat VERSION)
GIT_COMMIT = $(shell git rev-parse --short HEAD)
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
RUNNER = $(shell id -u -n)@$(shell hostname)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

all: lint mad

.PHONY: setup-dependencies
setup-dependencies:
	@# Setup vale.
	@wget https://github.com/errata-ai/vale/releases/download/v$(VALE_VERSION)/vale_$(VALE_VERSION)_Linux_64-bit.tar.gz && \
	mkdir -p assets && tar -xvzf vale_$(VALE_VERSION)_Linux_64-bit.tar.gz -C assets && \
	chmod +x $(ASSETS_DIR)vale && \
	$(ASSETS_DIR)vale sync
	@# Setup markdownfmt.
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) install github.com/Kunde21/markdownfmt/v3/cmd/markdownfmt@$(MARKDOWNFMT_VERSION)
	@# Setup golangci-lint.
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)


.PHONY: test-unit
test-unit:
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) test -v -race $(shell $(GO) list ./... | grep -v $(E2E_TEST_PKG))

.PHONY: test-e2e
test-e2e:
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) test -v -race $(E2E_TEST_PKG)

.PHONY: test
test: test-unit test-e2e

.PHONY: clean
clean:
	@rm -f mad
	@git clean -fxd

.make/vale: .vale.ini $(wildcard .vale/*) $(MD_FILES)
	@$(ASSETS_DIR)vale $(MD_FILES)
	@if [ $$? -eq 0 ]; then touch $@; fi

.make/markdownfmt: $(MD_FILES)
	@test -z "$(shell markdownfmt -l $(MD_FILES))" || (echo "\033[0;31mThe following files need to be formatted with 'markdownfmt -w -gofmt':" $(shell markdownfmt -l $(MD_FILES)) "\033[0m" && exit 1)
	@touch $@

.PHONY: lint-md
lint-md: .make/vale .make/markdownfmt

.make/gofmt: $(GO_FILES)
	@test -z "$(shell gofmt -l $(GO_FILES))" || (echo "\033[0;31mThe following files need to be formatted with 'gofmt -w':" $(shell gofmt -l $(GO_FILES)) "\033[0m" && exit 1)
	@if [ $$? -eq 0 ]; then touch $@; fi

.make/golangci-lint: $(GO_FILES)
	@golangci-lint run
	@if [ $$? -eq 0 ]; then touch $@; fi

.PHONY: lint-go
lint-go: .make/gofmt .make/golangci-lint

.PHONY: lint
lint: lint-md lint-go

.make/markdownfmt-fix: $(MD_FILES)
	@for file in $(MD_FILES); do markdownfmt -w -gofmt $$file || exit 1; done
	@touch $@

.PHONY: lint-md-fix
lint-md-fix: .make/vale .make/markdownfmt-fix

.make/gofmt-fix: $(GO_FILES)
	@gofmt -w . || exit 1
	@touch $@

.PHONY: lint-go-fix
lint-go-fix: .make/gofmt-fix .make/golangci-lint

.PHONY: lint-fix
lint-fix: lint-md-fix lint-go-fix

mad: $(GO_FILES)
	@GOOS=$(OS) GOARCH=$(ARCH) $(GO) build -ldflags "-s -w \
	-X ${COMMON}/version.Version=v${VERSION} \
	-X ${COMMON}/version.Revision=${GIT_COMMIT} \
	-X ${COMMON}/version.Branch=${BRANCH} \
	-X ${COMMON}/version.BuildUser=${RUNNER} \
	-X ${COMMON}/version.BuildDate=${BUILD_DATE}" \
	-o $@
