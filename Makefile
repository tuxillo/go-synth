# Makefile for dsynth-go

PROG=		dsynth
SRCS=		$(shell find . -name '*.go' -not -path './vendor/*')
VERSION=	2.0.0

PREFIX?=	/usr/local
BINDIR=		$(PREFIX)/bin
MANDIR=		$(PREFIX)/man/man1

GO?=		go
GOFLAGS=	-ldflags "-X main.Version=$(VERSION)"

all: build

build: $(PROG)

$(PROG): $(SRCS)
	$(GO) build $(GOFLAGS) -o $(PROG) .

install: $(PROG)
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(PROG) $(DESTDIR)$(BINDIR)/

clean:
	rm -f $(PROG)
	$(GO) clean

test:
	$(GO) test -v ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

# ==============================================================================
# VM Testing Infrastructure (DragonFlyBSD on QEMU/KVM)
# ==============================================================================
#
# Prerequisites:
#   - QEMU/KVM installed on host (qemu-system-x86_64)
#   - 20GB disk space for VM image
#   - ~300MB for DragonFlyBSD ISO
#
# First-time setup (run once):
#   1. make vm-setup      # Download ISO, create disk
#   2. make vm-install    # Boot VM for manual installation (10 min)
#   3. SSH to VM and run: ./scripts/vm/provision.sh
#   4. make vm-snapshot   # Save clean VM state
#
# Daily development workflow:
#   1. make vm-start      # Boot VM (30s)
#   2. Edit code locally
#   3. make vm-quick      # Sync + test Phase 4
#   4. make vm-stop       # Shut down VM
#
# See docs/testing/VM_TESTING.md for complete documentation.

VM_DIR=		vm
VM_SSH=		ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@localhost
VM_RSYNC=	rsync -avz --delete --exclude='.git' --exclude='vm/' -e "ssh -p 2222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"

# ------------------------------------------------------------------------------
# VM Lifecycle Management
# ------------------------------------------------------------------------------

vm-setup:
	@echo "==> Setting up VM environment..."
	@./scripts/vm/fetch-dfly-image.sh
	@./scripts/vm/create-disk.sh
	@echo ""
	@echo "Setup complete! Next steps:"
	@echo "  1. Run 'make vm-install' to boot VM for installation"
	@echo "  2. Install DragonFlyBSD manually (follow prompts)"
	@echo "  3. SSH to VM: ssh -p 2222 root@localhost"
	@echo "  4. Run provisioning: ./scripts/vm/provision.sh"
	@echo "  5. Run 'make vm-snapshot' to save clean state"

vm-install:
	@echo "==> Booting VM for DragonFlyBSD installation..."
	@echo "Follow the installation prompts. When done:"
	@echo "  1. SSH to VM: ssh -p 2222 root@localhost"
	@echo "  2. Run: ./scripts/vm/provision.sh"
	@echo "  3. Run: make vm-snapshot"
	@echo ""
	@./scripts/vm/start-vm.sh

vm-snapshot:
	@echo "==> Creating clean VM snapshot..."
	@./scripts/vm/snapshot-clean.sh
	@echo ""
	@echo "Clean snapshot saved! You can now:"
	@echo "  - Run 'make vm-restore' to reset to clean state"
	@echo "  - Run 'make vm-start' to boot VM normally"

vm-start:
	@echo "==> Starting DragonFlyBSD VM..."
	@./scripts/vm/start-vm.sh
	@echo ""
	@echo "VM is starting. Waiting for SSH..."
	@sleep 10
	@$(VM_SSH) 'echo "VM ready!"' || echo "VM not ready yet, wait a moment and try 'make vm-ssh'"

vm-stop:
	@echo "==> Stopping VM..."
	@./scripts/vm/stop-vm.sh

vm-destroy:
	@echo "==> WARNING: This will delete the VM and all data!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	@./scripts/vm/destroy-vm.sh

vm-restore:
	@echo "==> Restoring VM to clean snapshot..."
	@./scripts/vm/restore-vm.sh
	@echo ""
	@echo "VM restored! Run 'make vm-start' to boot."

vm-ssh:
	@$(VM_SSH)

vm-status:
	@echo "==> VM Status"
	@echo ""
	@if pgrep -f "qemu-system-x86_64.*dfly-vm.qcow2" > /dev/null; then \
		echo "VM Status: RUNNING"; \
		echo "PID: $$(pgrep -f 'qemu-system-x86_64.*dfly-vm.qcow2')"; \
		echo ""; \
		echo "SSH Access: ssh -p 2222 root@localhost"; \
		echo ""; \
		$(VM_SSH) 'uname -a; uptime' 2>/dev/null || echo "VM not responding to SSH yet"; \
	else \
		echo "VM Status: STOPPED"; \
		echo ""; \
		echo "Run 'make vm-start' to boot VM"; \
	fi

# ------------------------------------------------------------------------------
# VM Testing Targets
# ------------------------------------------------------------------------------

vm-sync:
	@echo "==> Syncing project to VM..."
	@$(VM_RSYNC) . root@localhost:/root/go-synth/

vm-build: vm-sync
	@echo "==> Building dsynth in VM..."
	@$(VM_SSH) 'cd /root/go-synth && make clean && make build'

vm-test-unit: vm-build
	@echo "==> Running unit tests in VM..."
	@$(VM_SSH) 'cd /root/go-synth && go test -v -short ./...'

vm-test-integration: vm-build
	@echo "==> Running integration tests in VM..."
	@$(VM_SSH) 'cd /root/go-synth && go test -v -run="TestIntegration" ./...'

vm-test-phase4: vm-build
	@echo "==> Running Phase 4 tests in VM (requires root + mount)..."
	@$(VM_SSH) 'cd /root/go-synth && doas go test -v ./internal/worker/...'

vm-test-e2e: vm-build
	@echo "==> Running E2E tests in VM..."
	@$(VM_SSH) 'cd /root/go-synth && go test -v -tags=e2e ./...'

vm-test-all: vm-build
	@echo "==> Running ALL tests in VM..."
	@$(VM_SSH) 'cd /root/go-synth && go test -v ./... && doas go test -v ./internal/worker/...'

vm-quick: vm-sync
	@echo "==> Quick test cycle (sync + Phase 4 tests)..."
	@$(VM_SSH) 'cd /root/go-synth && make build && doas go test -v ./internal/worker/...'

# ------------------------------------------------------------------------------
# VM Help
# ------------------------------------------------------------------------------

vm-help:
	@echo "VM Testing Infrastructure - DragonFlyBSD on QEMU/KVM"
	@echo ""
	@echo "FIRST-TIME SETUP:"
	@echo "  vm-setup         Download ISO, create disk"
	@echo "  vm-install       Boot VM for manual installation"
	@echo "  vm-snapshot      Save clean VM state (after provisioning)"
	@echo ""
	@echo "LIFECYCLE:"
	@echo "  vm-start         Start VM"
	@echo "  vm-stop          Stop VM"
	@echo "  vm-destroy       Delete VM and all data"
	@echo "  vm-restore       Reset VM to clean snapshot"
	@echo "  vm-ssh           SSH into VM"
	@echo "  vm-status        Show VM status and info"
	@echo ""
	@echo "TESTING:"
	@echo "  vm-sync          Sync project files to VM"
	@echo "  vm-build         Build dsynth in VM"
	@echo "  vm-test-unit     Run unit tests"
	@echo "  vm-test-integration  Run integration tests"
	@echo "  vm-test-phase4   Run Phase 4 tests (mount, chroot)"
	@echo "  vm-test-e2e      Run end-to-end tests"
	@echo "  vm-test-all      Run all tests"
	@echo "  vm-quick         Quick cycle: sync + Phase 4 tests"
	@echo ""
	@echo "HELP:"
	@echo "  vm-help          Show this help"
	@echo ""
	@echo "See docs/testing/VM_TESTING.md for complete documentation."

.PHONY: all build install clean test fmt vet
.PHONY: vm-setup vm-install vm-snapshot vm-start vm-stop vm-destroy vm-restore
.PHONY: vm-ssh vm-status vm-sync vm-build vm-test-unit vm-test-integration
.PHONY: vm-test-phase4 vm-test-e2e vm-test-all vm-quick vm-help