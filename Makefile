.PHONY: build clean test run deps deb deb-clean install help

BINARY_NAME=prox-mds
VERSION?=dev
BUILD_DIR=bin

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

deps: ## Download dependencies
	@go mod download
	@go mod tidy

build: clean ## Build the binary
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "-X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) .

clean: ## Remove build artifacts
	@rm -rf $(BUILD_DIR)
	@go clean

test: ## Run tests
	@go test -v ./...

run: build ## Build and run the binary
	@sudo $(BUILD_DIR)/$(BINARY_NAME) -config /etc/prox-mds/config.yaml

install: build ## Install binary to system
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to /usr/local/bin/"

deb: ## Build Debian package natively
	@chmod +x debian/postinst debian/prerm
	@dpkg-buildpackage -b -us -uc

deb-clean: ## Clean Debian build artifacts
	@rm -rf debian/prox-mds debian/*.substvars debian/files
	@rm -f ../prox-mds*.deb ../prox-mds*.changes ../prox-mds*.dsc
	@rm -f ../deb-packages/*.deb ../deb-packages/*.changes ../deb-packages/*.dsc

