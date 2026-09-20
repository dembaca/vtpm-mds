.PHONY: build clean test run deps deb deb-local deb-clean install help lab-setup lab-e2e lab-devid-e2e

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
	@go build -ldflags "-X main.Version=$(VERSION)" -o $(BUILD_DIR)/$(DEVID_CLIENT) ./cmd/devid-enroll
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

# Stage a copy so dpkg-buildpackage can write the .deb to a writable parent
# (the source tree's real parent is often not writable in Cloud Agent VMs).
DEB_STAGE := $(CURDIR)/.deb-build
DEB_DIST := $(CURDIR)/dist

deb: ## Build host + guest Debian packages (dpkg-buildpackage → dist/*.deb)
	@test -d debian
	@if [ -d "$(DEB_STAGE)" ]; then find "$(DEB_STAGE)" -type f -exec setfacl -m mask::rwx {} + 2>/dev/null || true; fi
	@if [ -d "$(DEB_STAGE)" ]; then chmod -R u+w "$(DEB_STAGE)" 2>/dev/null || true; fi
	@rm -rf $(DEB_STAGE)
	@mkdir -p $(DEB_STAGE)/src $(DEB_DIST)
	@tar -C $(CURDIR) \
		--exclude=.deb-build --exclude=dist --exclude=.git --exclude=bin \
		--exclude=.go-workdir --exclude=debian/.debhelper --exclude=debian/vtpm-mds \
		--exclude=debian/devid-enroll \
		-cf - . | tar -C $(DEB_STAGE)/src -xf -
	@cd $(DEB_STAGE)/src && dpkg-buildpackage -b -us -uc
	@cp -f $(DEB_STAGE)/*.deb $(DEB_STAGE)/*.changes $(DEB_STAGE)/*.buildinfo $(DEB_DIST)/
	@echo "Built Debian packages (host $(BINARY_NAME), guest $(DEVID_CLIENT)):"
	@ls -lh $(DEB_DIST)/*.deb

deb-local: ## Build git-stamped local .debs (version <changelog>+git<date>.<sha>[.dirty])
	@test -d debian
	@./scripts/deb-local.sh

deb-clean: ## Clean Debian build artifacts
	@if [ -d "$(DEB_STAGE)" ]; then find "$(DEB_STAGE)" -type f -exec setfacl -m mask::rwx {} + 2>/dev/null || true; fi
	@if [ -d .go-workdir ]; then find .go-workdir -type f -exec setfacl -m mask::rwx {} + 2>/dev/null || true; fi
	@# Go leaves the module cache mode 0555, including directories, so rm -rf
	@# fails until the write bit is back on the directories themselves.
	@for d in "$(DEB_STAGE)" .go-workdir; do [ -d "$$d" ] && chmod -R u+w "$$d" 2>/dev/null || true; done
	@rm -rf debian/prox-mds debian/vtpm-mds debian/devid-enroll debian/.debhelper debian/.gocache debian/.gomod debian/.gopath .go-workdir
	@rm -f debian/*.substvars debian/files debian/*.debhelper.log debian/*.debhelper
	@rm -rf $(DEB_STAGE) $(DEB_DIST)
	@rm -f ../prox-mds*.deb ../vtpm-mds*.deb ../prox-mds*.changes ../vtpm-mds*.changes ../prox-mds*.dsc ../vtpm-mds*.dsc
	@rm -f ../devid-enroll*.deb ../devid-enroll*.changes ../devid-enroll*.dsc
	@rm -f ../prox-mds_*.buildinfo ../vtpm-mds_*.buildinfo ../prox-mds_*.build ../vtpm-mds_*.build
	@rm -f ../devid-enroll_*.buildinfo ../devid-enroll_*.build
	@rm -f ../deb-packages/*.deb ../deb-packages/*.changes ../deb-packages/*.dsc

lab-setup: ## Prepare QEMU/netns MDS lab host networking
	@./scripts/qemu-lab/setup-host.sh

lab-e2e: ## Run netns IMDS smoke test (vtpm-mds must be running)
	@./scripts/qemu-lab/e2e-netns.sh

lab-devid-e2e: ## Boot QEMU+swtpm guest and enroll SPIRE DevID materials
	@./scripts/qemu-lab/e2e-devid-guest.sh
