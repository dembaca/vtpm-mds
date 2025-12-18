.PHONY: build clean test run deps lint docker-build docker-run deb install help deb-docker

BINARY_NAME=prox-mds
VERSION?=dev
BUILD_DIR=bin
UNAME_S := $(shell uname -s)

# Detect if we're on macOS or Linux
ifeq ($(UNAME_S),Darwin)
    BUILD_DEB_CMD = make deb-docker
else ifeq ($(UNAME_S),Linux)
    BUILD_DEB_CMD = deb-native
else
    BUILD_DEB_CMD = make deb-docker
endif

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
	@go build -ldflags "-X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/prox-mds

clean: ## Remove build artifacts
	@rm -rf $(BUILD_DIR)
	@go clean

test: ## Run tests
	@go test -v ./...

run: build ## Build and run the binary
	@sudo $(BUILD_DIR)/$(BINARY_NAME) -config /etc/prox-mds/config.yaml

docker-build: ## Build Docker image
	@docker build -t prox-mds:$(VERSION) .

docker-run: docker-build ## Build and run Docker container
	@docker run --rm --cap-add=NET_ADMIN --network host prox-mds:$(VERSION)

install: build ## Install binary to system
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to /usr/local/bin/"

deb: ## Build Debian package (auto-detects platform)
	@echo "Detected OS: $(UNAME_S)"
	@$(BUILD_DEB_CMD)

deb-native: ## Build Debian package natively (Linux)
	@chmod +x debian/postinst debian/prerm
	@dpkg-buildpackage -b -us -uc

deb-docker: ## Build Debian package in Docker container (macOS/cross-platform)
	@echo "Building Debian package in Docker container for Linux AMD64..."
	@chmod +x debian/postinst debian/prerm
	@mkdir -p ../deb-packages
	@docker build -f Dockerfile.debbuild -t prox-mds-debbuild:latest .
	@docker run --rm \
		-v "$(CURDIR)/..:/build" \
		-v "$(CURDIR):/build/prox-mds" \
		-w /build/prox-mds \
		-e DEB_BUILD_OPTIONS=nocheck \
		-e DEB_BUILD_ARCH=amd64 \
		-e ARCH=amd64 \
		prox-mds-debbuild:latest \
		dpkg-buildpackage -aamd64 -d -b -us -uc
	@mkdir -p ../deb-packages
	@mv ../prox-mds*.deb ../deb-packages/ 2>/dev/null || true
	@mv ../prox-mds*.changes ../deb-packages/ 2>/dev/null || true
	@mv ../prox-mds*.dsc ../deb-packages/ 2>/dev/null || true
	@rm -f ../prox-mds*.deb ../prox-mds*.changes ../prox-mds*.dsc 2>/dev/null || true
	@echo "Package built in: ../deb-packages/"
	@ls -lh ../deb-packages/*.deb 2>/dev/null || true

deb-clean: ## Clean Debian build artifacts
	@rm -rf debian/prox-mds debian/*.substvars debian/files
	@rm -f ../prox-mds*.deb ../prox-mds*.changes ../prox-mds*.dsc
	@rm -f ../deb-packages/*.deb ../deb-packages/*.changes ../deb-packages/*.dsc

sync: ## Sync code to Proxmox hogan (set PROXMOX_HOST env var to override)
	@./scripts/sync-to-proxmox.sh $(PROXMOX_HOST)

remote-test: ## Run tests on remote Proxmox host hogan
	@./scripts/remote-test.sh $(PROXMOX_HOST)

