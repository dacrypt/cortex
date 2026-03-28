.PHONY: proto build test test-pdf run clean install-pdf-tools diagnose

# Proto generation
PROTO_DIR := api/proto
GEN_DIR := api/gen

proto:
	@echo "Generating protobuf code..."
	@mkdir -p $(GEN_DIR)/cortex/v1
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/cortex/v1/*.proto

# Build
build:
	go build -o cortexd ./cmd/cortexd

# Install PDF metadata extraction tools (required for PDF tests)
install-pdf-tools:
	@echo "Installing PDF metadata extraction tools..."
	@./scripts/install-pdf-tools.sh

# Test
test:
	go test -v ./...

# Test PDF extractor (requires pdfinfo and exiftool)
test-pdf: install-pdf-tools
	go test -v ./internal/infrastructure/metadata -run TestPDFExtractor

# Run
run: build
	./cortexd

# Build diagnose-consistency tool
diagnose-build:
	go build -o diagnose-consistency ./cmd/diagnose-consistency

# Run diagnose-consistency (usage: make diagnose DB=path/to/db.sqlite [WORKSPACE=workspace-id])
diagnose: diagnose-build
	@if [ -z "$(DB)" ]; then \
		echo "Error: DB variable is required. Usage: make diagnose DB=path/to/index.sqlite [WORKSPACE=workspace-id]"; \
		exit 1; \
	fi
	@if [ -z "$(WORKSPACE)" ]; then \
		./diagnose-consistency -db $(DB); \
	else \
		./diagnose-consistency -db $(DB) -workspace $(WORKSPACE); \
	fi

# Clean
clean:
	rm -f cortexd diagnose-consistency
	rm -rf $(GEN_DIR)
