# Developer tasks for Crabby. End users never build from source — they use the
# install scripts (see README). These targets are for contributors only.

MODULE   := github.com/marioolf/crabby
BINARY   := crabby
BIN_DIR  := bin
DIST_DIR := dist
VERSION  := $(shell cat VERSION)
LDFLAGS  := -s -w -X $(MODULE)/internal/version.Version=$(VERSION)

# os/arch pairs shipped in a release.
PLATFORMS := linux/amd64 linux/arm64 windows/amd64 windows/arm64

.PHONY: help build test fmt lint vet release clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

build: ## Compile the host binary into bin/
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) ./cmd/crabby

test: ## Run the test suite
	go test ./...

fmt: ## Format all Go code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: vet ## Run go vet and check formatting
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needs to run on:"; echo "$$unformatted"; exit 1; \
	fi

release: clean ## Cross-compile and package every platform into dist/
	@mkdir -p $(DIST_DIR)
	@set -e; for platform in $(PLATFORMS); do \
		os=$${platform%%/*}; arch=$${platform##*/}; \
		if [ "$$os" = "windows" ]; then pkg=./cmd/crabby-windows; bin=$(BINARY).exe; \
		else pkg=./cmd/crabby; bin=$(BINARY); fi; \
		echo ">> building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$$bin $$pkg; \
		if [ "$$os" = "windows" ]; then \
			zipname=$(BINARY)_$${os}_$${arch}.zip; \
			if command -v zip >/dev/null 2>&1; then \
				(cd $(DIST_DIR) && zip -q $$zipname $$bin); \
			else \
				(cd $(DIST_DIR) && python3 -c "import sys,zipfile; z=zipfile.ZipFile(sys.argv[1],'w',zipfile.ZIP_DEFLATED); z.write(sys.argv[2]); z.close()" $$zipname $$bin); \
			fi; \
			rm -f $(DIST_DIR)/$$bin; \
		else \
			(cd $(DIST_DIR) && tar -czf $(BINARY)_$${os}_$${arch}.tar.gz $$bin && rm -f $$bin); \
		fi; \
	done
	@cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "Artifacts in $(DIST_DIR)/:" && ls -1 $(DIST_DIR)

clean: ## Remove build output
	rm -rf $(BIN_DIR) $(DIST_DIR)
