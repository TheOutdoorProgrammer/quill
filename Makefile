BIN := quill
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
APPLE_SIGNING_BIN := quill-apple-signing
APPLE_SIGNING_PLATFORMS := darwin/amd64 darwin/arm64

# The toolchain is pinned exactly, not as a minimum, so that a dist built on a
# laptop is byte for byte what CI rebuilds. Without this, `make verify-dist`
# fails whenever the two machines are on different patch releases of Go.
export GOTOOLCHAIN := go1.27.0

# -buildvcs=false keeps the commit out of the binary. With it on, dist would
# change on every commit and stop being a function of the source alone.
BUILD_FLAGS := -trimpath -buildvcs=false -ldflags '-s -w'

.PHONY: all
all: test dist

.PHONY: test
test:
	go test -race ./...

.PHONY: lint
lint:
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	golangci-lint run

.PHONY: build
build:
	go build -o $(BIN) ./cmd/$(BIN)

# dist is what the action actually runs. It is committed, so every rebuild has
# to land identical bytes for an unchanged source tree.
.PHONY: dist
dist:
	@mkdir -p dist apple-signing/dist
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "building dist/$(BIN)-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build $(BUILD_FLAGS) -o dist/$(BIN)-$$os-$$arch ./cmd/$(BIN) || exit 1; \
	done
	@cd dist && shasum -a 256 $(BIN)-* > SHA256SUMS
	@chmod +x dist/$(BIN)-*
	@for platform in $(APPLE_SIGNING_PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		echo "building apple-signing/dist/$(APPLE_SIGNING_BIN)-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build $(BUILD_FLAGS) -o apple-signing/dist/$(APPLE_SIGNING_BIN)-$$os-$$arch ./cmd/$(APPLE_SIGNING_BIN) || exit 1; \
	done
	@cd apple-signing/dist && shasum -a 256 $(APPLE_SIGNING_BIN)-* > SHA256SUMS
	@chmod +x apple-signing/dist/$(APPLE_SIGNING_BIN)-*

# The gate that stops a committed binary drifting from the source beside it.
.PHONY: verify-dist
verify-dist: dist
	@git diff --exit-code -- dist/ apple-signing/dist/ || { \
		echo "::error::dist/ is stale. Run 'make dist' and commit the result."; \
		exit 1; \
	}
	@echo "committed binaries match the source"

.PHONY: clean
clean:
	rm -rf $(BIN) dist/$(BIN)-* dist/SHA256SUMS apple-signing/dist/
