.PHONY: build clean test run deps deb deb-clean install help lab-setup lab-e2e lab-devid-e2e

BINARY_NAME=vtpm-mds
LEGACY_ALIAS=prox-mds
QEMU_ALIAS=qemu-mds
DEVID_CLIENT=devid-enroll
VERSION?=dev
BUILD_DIR=bin

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

deps: ## Download dependencies
	@go mod download
	@go mod tidy

build: ## Build vtpm-mds (+ legacy aliases) and devid-enroll client
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags "-X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@cp -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(LEGACY_ALIAS)
	@cp -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(QEMU_ALIAS)
	@go build -o $(BUILD_DIR)/$(DEVID_CLIENT) ./cmd/devid-enroll
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME) (aliases: $(LEGACY_ALIAS), $(QEMU_ALIAS)) $(BUILD_DIR)/$(DEVID_CLIENT)"

clean: ## Remove build artifacts
	@rm -f $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(LEGACY_ALIAS) $(BUILD_DIR)/$(QEMU_ALIAS) $(BUILD_DIR)/$(DEVID_CLIENT) 2>/dev/null || true
	@go clean

test: ## Run tests
	@go test -v ./...

run: build ## Build and run the binary
	@sudo $(BUILD_DIR)/$(BINARY_NAME) -config /etc/vtpm-mds/config.yaml

install: build ## Install binary to system
	@sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@sudo cp $(BUILD_DIR)/$(LEGACY_ALIAS) /usr/local/bin/$(LEGACY_ALIAS)
	@sudo cp $(BUILD_DIR)/$(QEMU_ALIAS) /usr/local/bin/$(QEMU_ALIAS)
	@sudo cp $(BUILD_DIR)/$(DEVID_CLIENT) /usr/local/bin/$(DEVID_CLIENT)
	@echo "Installed $(BINARY_NAME) (+ aliases) and $(DEVID_CLIENT) to /usr/local/bin/"

deb: ## Build Debian package natively
	@chmod +x debian/postinst debian/prerm
	@dpkg-buildpackage -b -us -uc

deb-clean: ## Clean Debian build artifacts
	@rm -rf debian/prox-mds debian/vtpm-mds debian/*.substvars debian/files
	@rm -f ../prox-mds*.deb ../vtpm-mds*.deb ../prox-mds*.changes ../vtpm-mds*.changes ../prox-mds*.dsc ../vtpm-mds*.dsc
	@rm -f ../deb-packages/*.deb ../deb-packages/*.changes ../deb-packages/*.dsc

lab-setup: ## Prepare QEMU/netns MDS lab host networking
	@./scripts/qemu-lab/setup-host.sh

lab-e2e: ## Run netns IMDS smoke test (vtpm-mds must be running)
	@./scripts/qemu-lab/e2e-netns.sh

lab-devid-e2e: ## Boot QEMU+swtpm guest and enroll SPIRE DevID materials
	@./scripts/qemu-lab/e2e-devid-guest.sh
