# core-module aggregator Makefile.
#
# Delegates build/test/clean across model/, api/, and gui/ sub-projects,
# and provides `preview` to run the Ladle component workbench against gui/.
#
# Sub-project Makefiles (model/, api/) are owned by @sdlcforge/gen-make;
# do not hand-edit them. This file is the module-level aggregator and is
# hand-maintained.
#
# Targets:
#   build    — build model, api, and gui
#   test     — run unit tests on model and api; typecheck gui
#   clean    — remove build artifacts from model, api, and gui
#   preview  — run the Ladle workbench for gui (localhost:61000)
#   help     — list available targets

ifeq ($(filter undefine override,$(value .FEATURES)),)
$(error GNU make 4.x+ required; on macOS: brew install make && use gmake)
endif

.DEFAULT_GOAL := build

GO_SUBPROJECTS := model api
GUI_DIR := gui

.PHONY: build
build: ## Build model, api, and gui
	@for d in $(GO_SUBPROJECTS); do \
		echo "==> build: core-module/$$d"; \
		$(MAKE) --no-print-directory -C $$d build; \
	done
	@echo "==> build: core-module/$(GUI_DIR)"
	@cd $(GUI_DIR) && bun run build

.PHONY: test
test: ## Test model and api; typecheck gui
	@for d in $(GO_SUBPROJECTS); do \
		echo "==> test: core-module/$$d"; \
		$(MAKE) --no-print-directory -C $$d test; \
	done
	@echo "==> typecheck: core-module/$(GUI_DIR)"
	@cd $(GUI_DIR) && bun run typecheck

.PHONY: clean
clean: ## Clean model, api, and gui build artifacts
	@for d in $(GO_SUBPROJECTS); do \
		echo "==> clean: core-module/$$d"; \
		$(MAKE) --no-print-directory -C $$d clean; \
	done
	@echo "==> clean: core-module/$(GUI_DIR)"
	@cd $(GUI_DIR) && bun run clean

.PHONY: preview
preview: ## Run Ladle component workbench for gui (localhost:61000)
	@echo "==> preview: core-module/$(GUI_DIR) — installing deps if needed"
	@cd $(GUI_DIR) && bun install --silent
	@echo "==> preview: starting Ladle on http://localhost:61000"
	@cd $(GUI_DIR) && bun run dev

.PHONY: help
help: ## Show this help
	@echo "core-module — module-level Makefile"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} \
		/^[a-zA-Z0-9_.\-]+:.*?## / { printf "  %-10s %s\n", $$1, $$2 }' \
		$(MAKEFILE_LIST) | sort
	@echo ""
	@echo "Sub-projects: $(GO_SUBPROJECTS) $(GUI_DIR)"
