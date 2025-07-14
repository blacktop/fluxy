.PHONY: bump
bump:
	@echo "🚀 Bumping Version"
	git tag $(shell svu patch)
	git push --tags

.PHONY: build
build:
	@echo "🚀 Building Version $(shell svu current)"
	go build -o fluxy main.go

.PHONY: release
release:
	@echo "🚀 Releasing Version $(shell svu current)"
	goreleaser build --id default --clean --snapshot --single-target --output dist/fluxy

.PHONY: vhs
vhs: release
	@echo "📼 VHS Recording"
	@echo "Please ensure you have the 'vhs' command installed."
	vhs < vhs.tape	

.PHONY: work-termimg
work-termimg: ## Work on go-termimg package
	@echo " > Working on go-termimg package"
	@go work init
	@go work use . ../go-termimg

.PHONY: work-clean
work-clean: ## Clean up go work
	@echo " > 🧼 Clean go.work"
	@rm go.work || true
	@rm go.work.sum || true

.PHONY: debug
debug: ## Debug in another terminal
	@echo " > Debugging"
	@dlv debug --headless --listen=:2345 --api-version=2 main.go -- --model schnell --output test