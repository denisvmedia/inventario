# Inventario preview-env infra orchestrator (#1853).
#
# Most targets require VM=user@host (the bootstrap target VM). The orchestration
# always runs from the laptop; nothing runs locally that needs root. The
# `preflight` target is the exception — it only checks laptop-side tooling and
# does not need VM.
#
# Required laptop tools: ssh, scp, sops, age. Helm/kubectl are nice-to-have
# (the installer ships them on the VM via vcluster standalone).
#
# See infra/README.md for the operating guide.

SHELL := /bin/bash

# Resolve script paths relative to this Makefile, not the caller's cwd, so
# `make -C infra bootstrap` (as the README documents) and `make -f infra/Makefile`
# both work. Without this the recipes' literal `infra/vm/...` paths would only
# work when invoked from the repo root and would 404 under `-C infra`.
THIS_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

VM ?=
SVC ?= vcluster.service

REQUIRE_VM = @[ -n "$(VM)" ] || { echo "ERROR: VM is required, e.g. make $@ VM=user@host" >&2; exit 1; }

.PHONY: help preflight bootstrap upgrade recover destroy status logs shell

help:
	@echo "Inventario preview-env infra targets (VM=user@host required except preflight)"
	@echo ""
	@echo "  bootstrap    Bring a clean Ubuntu 26.04 VM up to a working preview-env host."
	@echo "  upgrade      Re-run bootstrap; idempotent. Use for vcluster/argocd version bumps."
	@echo "  recover      Alias for bootstrap; intended after total VM loss + 'ssh-copy-id'."
	@echo "  destroy      Stop services and remove vcluster data dir. Tailnet membership preserved."
	@echo "  status       Show vcluster+tailscaled state and kubectl get nodes,pods -A."
	@echo "  logs         Tail journalctl for SVC (default: vcluster.service)."
	@echo "               Example: make logs VM=... SVC=tailscaled.service"
	@echo "  shell        ssh into the VM."
	@echo ""
	@echo "Laptop prereqs: ssh, scp, sops, age. Run 'make preflight' to verify."

preflight:
	@missing=""; \
	for c in ssh scp sops age; do \
	  command -v $$c >/dev/null 2>&1 || missing="$$missing $$c"; \
	done; \
	if [ -n "$$missing" ]; then \
	  echo "Missing tools:$$missing" >&2; \
	  echo "  macOS: brew install$$missing" >&2; \
	  echo "  Debian/Ubuntu: apt-get install$$missing" >&2; \
	  exit 1; \
	fi; \
	echo "Laptop prereqs OK"

bootstrap: preflight
	$(REQUIRE_VM)
	bash "$(THIS_DIR)vm/bootstrap.sh" "$(VM)"

upgrade: bootstrap

recover: bootstrap

destroy:
	$(REQUIRE_VM)
	bash "$(THIS_DIR)vm/destroy.sh" "$(VM)"

status:
	$(REQUIRE_VM)
	@ssh "$(VM)" 'set -e; \
	  printf "\n=== systemd units ===\n"; \
	  systemctl is-active vcluster.service tailscaled.service || true; \
	  printf "\n=== tailscale ===\n"; \
	  tailscale status --self=true --peers=false 2>/dev/null || true; \
	  printf "\n=== kubectl get nodes ===\n"; \
	  sudo /usr/local/bin/kubectl get nodes -o wide 2>/dev/null || echo "(kubectl unavailable)"; \
	  printf "\n=== pods (all namespaces) ===\n"; \
	  sudo /usr/local/bin/kubectl get pods -A 2>/dev/null || true'

logs:
	$(REQUIRE_VM)
	ssh "$(VM)" "sudo journalctl -u $(SVC) -f --no-pager"

shell:
	$(REQUIRE_VM)
	ssh "$(VM)"
