# Makefile — fast-levenshtein (CalVer + pkg.go.dev)
# Versionado fecha: v2026.08.24 (alias) + v2.2026.08.24 (canónico para /v2)
# Uso:
#   make test       # tests rápidos
#   make ci         # gate completo (vet + race + cover)
#   make push       # git push main
#   make tags       # crea tag fecha + push + publica en proxy.golang.org
#   make publish TAG=v2.2026.08.24  # fuerza proxy refresh de un tag existente
#   make version    # muestra VERSION/TAG calculados

SHELL := /usr/bin/env bash
GO ?= go
MODULE := github.com/yukiteruamano/fast-levenshtein/v2
REMOTE ?= origin
BRANCH ?= main

# CalVer UTC: permite override -> make tags VERSION=2026.08.25
VERSION ?= $(shell date -u +%Y.%m.%d)
TAG := v2.$(VERSION)
TAG_ALIAS := v$(VERSION)

# Colores
GREEN  := \033[32m
YELLOW := \033[33m
RED    := \033[31m
RESET  := \033[0m

.PHONY: help test vet race cover bench fuzz ci version tag tags push publish release clean check-dirty check-ci

help: ## Muestra ayuda
	@echo -e "$(GREEN)fast-levenshtein — Makefile$(RESET)"
	@echo -e "  MODULE=$(MODULE)  BRANCH=$(BRANCH)  REMOTE=$(REMOTE)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS=":.*?## "}; {printf "  $(YELLOW)%-12s$(RESET) %s\n", $$1, $$2}'
	@echo ""
	@echo -e "Ejemplos:"
	@echo -e "  make ci                          # gate local (vet+race+cover)"
	@echo -e "  make push                        # git push origin main"
	@echo -e "  make tags                        # tag fecha hoy + push + proxy refresh"
	@echo -e "  make tags VERSION=2026.08.25     # tag fecha específica"
	@echo -e "  make publish TAG=v2.2026.08.24   # refrescar pkg.go.dev"
	@echo -e "  make release                     # ci + tags"

# ---------------------------------------------------------------------------
# Tests / calidad
# ---------------------------------------------------------------------------
test: ## go test corto
	$(GO) test ./...

vet: ## go vet
	$(GO) vet ./...

race: ## go test -race
	$(GO) test -race -count=1 ./...

cover: ## cobertura + HTML
	$(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out | tail -n 20
	@echo -e "$(GREEN)Generado coverage.out$(RESET) — ver HTML con: go tool cover -html=coverage.out"

bench: ## benchmarks
	$(GO) test -bench=. -benchmem -count=1 ./...

fuzz: ## fuzz FuzzDistance 30s
	$(GO) test -run=^$$ -fuzz=FuzzDistance -fuzztime=30s ./...

ci: vet race cover ## Gate CI local (vet + race + cover)
	@echo -e "$(GREEN)✓ CI local OK$(RESET)"

# ---------------------------------------------------------------------------
# Versionado
# ---------------------------------------------------------------------------
version: ## Muestra VERSION/TAG calculados y tags existentes
	@echo "VERSION  = $(VERSION)"
	@echo "TAG      = $(TAG)  (canónico /v2, usado por pkg.go.dev)"
	@echo "TAG_ALIAS= $(TAG_ALIAS)  (alias legible)"
	@echo "MODULE   = $(MODULE)"
	@echo ""
	@echo "Tags locales que coinciden con fecha:"
	@git tag --list "v*$(VERSION)*" | sort -V || true
	@echo ""
	@echo "Tags remotos que coinciden:"
	@git ls-remote --tags $(REMOTE) 2>/dev/null | grep -E "v2?\.?$(VERSION)" | awk '{print $$2}' | sed 's|refs/tags/||' | sort -V || echo "(ninguno)"

check-dirty:
	@if [ -n "$$(git status --porcelain)" ] && [ "$(FORCE)" != "1" ]; then \
		echo -e "$(RED)✗ Working tree sucio. Commit/stash o usa FORCE=1$(RESET)"; \
		git status --short; \
		exit 1; \
	fi

check-ci: ## Valida go.mod y vet rápido antes de taggear
	@$(GO) vet ./... || (echo -e "$(RED)✗ go vet falló$(RESET)"; exit 1)
	@$(GO) test -count=1 ./... >/dev/null || (echo -e "$(RED)✗ go test falló$(RESET)"; exit 1)

# ---------------------------------------------------------------------------
# Git helpers
# ---------------------------------------------------------------------------
push: check-dirty ## git push BRANCH a REMOTE (default origin main)
	@echo -e "$(YELLOW)→ git push $(REMOTE) $(BRANCH)$(RESET)"
	@current=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$current" != "$(BRANCH)" ]; then \
		echo -e "$(YELLOW)⚠ Estás en rama '$$current', pusheando '$(BRANCH)' igualmente$(RESET)"; \
	fi
	@git push $(REMOTE) $(BRANCH)
	@echo -e "$(GREEN)✓ push OK$(RESET)"
	@if git tag --points-at HEAD | grep -q .; then \
		echo -e "$(YELLOW)ℹ Hay tags locales en HEAD no pusheados. Usa 'make tags' o 'git push $(REMOTE) --tags'$(RESET)"; \
	fi

# tag: crea tags locales (canónico + alias) con manejo de colisión .1, .2
tag: check-ci ## Crea tags locales v2.YYYY.MM.DD + vYYYY.MM.DD (con sufijo .1 si colisiona)
	@set -e; \
	TAG="$(TAG)"; ALIAS="$(TAG_ALIAS)"; \
	# — resuelve colisión para TAG canónico —; \
	ORIG_TAG="$$TAG"; i=0; \
	while git rev-parse "$$TAG" >/dev/null 2>&1 || git ls-remote --tags $(REMOTE) 2>/dev/null | grep -q "refs/tags/$$TAG$$"; do \
		i=$$((i+1)); TAG="$$ORIG_TAG.$$i"; ALIAS="$(TAG_ALIAS).$$i"; \
		if [ $$i -gt 20 ]; then echo -e "$(RED)Demasiadas colisiones$(RESET)"; exit 1; fi; \
	done; \
	if [ "$$TAG" != "$(TAG)" ]; then echo -e "$(YELLOW)⚠ $(TAG) existía, usando $$TAG$(RESET)"; fi; \
	echo -e "$(YELLOW)→ Creando tag $$TAG$(RESET)"; \
	git tag -a "$$TAG" -m "release $$TAG — $(MODULE)"; \
	echo -e "$(YELLOW)→ Creando alias $$ALIAS$(RESET)"; \
	git tag -a "$$ALIAS" -m "alias $$ALIAS for $$TAG — $(MODULE)"; \
	echo -e "$(GREEN)✓ Tags creados: $$TAG + $$ALIAS$(RESET)"; \
	echo "$$TAG" > .last-tag; echo "$$ALIAS" >> .last-tag; \
	echo -e "  Verifica: $(YELLOW)git tag --list | grep $(VERSION)$(RESET)"; \
	echo -e "  Publica:  $(YELLOW)make publish TAG=$$TAG$(RESET) o $(YELLOW)make tags$(RESET) para push+publish"

tags: tag ## Crea tags + push a REMOTE + publica en proxy.golang.org
	@set -e; \
	if [ -f .last-tag ]; then \
		TAGS=$$(cat .last-tag | tr '\n' ' '); \
	else \
		TAGS="$(TAG) $(TAG_ALIAS)"; \
	fi; \
	echo -e "$(YELLOW)→ git push $(REMOTE) $$TAGS$(RESET)"; \
	git push $(REMOTE) $$TAGS; \
	echo -e "$(GREEN)✓ tags pusheados$(RESET)"; \
	CANONICAL=$$(echo $$TAGS | awk '{print $$1}'); \
	echo -e "$(YELLOW)→ Publicando $$CANONICAL en proxy.golang.org$(RESET)"; \
	$(MAKE) publish TAG=$$CANONICAL; \
	rm -f .last-tag

publish: ## Refresca pkg.go.dev vía proxy: make publish TAG=v2.2026.08.24
	@set -e; \
	TAG="$${TAG:-$(TAG)}"; \
	if [ -z "$$TAG" ]; then echo -e "$(RED)TAG requerido: make publish TAG=v2.2026.08.24$(RESET)"; exit 1; fi; \
	if ! git rev-parse "$$TAG" >/dev/null 2>&1 && ! git ls-remote --tags $(REMOTE) 2>/dev/null | grep -q "refs/tags/$$TAG$$"; then \
		echo -e "$(RED)✗ Tag $$TAG no existe local ni remoto$(RESET)"; exit 1; \
	fi; \
	echo -e "$(YELLOW)→ GOPROXY=proxy.golang.org go list -m $(MODULE)@$$TAG$(RESET)"; \
	GOPROXY=proxy.golang.org go list -m $(MODULE)@$$TAG; \
	echo ""; \
	echo -e "$(YELLOW)→ Verificando proxy info$(RESET)"; \
	curl -fsSL "https://proxy.golang.org/$(MODULE)/@v/$$TAG.info" | head -c 500; echo ""; \
	echo ""; \
	echo -e "$(GREEN)✓ Publicado. Verifica en:$(RESET)"; \
	echo -e "  https://pkg.go.dev/$(MODULE)@$$TAG"; \
	echo -e "  https://proxy.golang.org/$(MODULE)/@v/$$TAG.info"; \
	echo -e "  https://proxy.golang.org/$(MODULE)/@v/list  (debe incluir $$TAG)"

release: ci tags ## Flujo completo: ci + tags (vet+race+cover → tag → push → publish)
	@echo -e "$(GREEN)✓ release completo$(RESET)"

clean: ## Limpia artefactos
	rm -f coverage.out coverage.html .last-tag
	@echo -e "$(GREEN)✓ clean$(RESET)"
