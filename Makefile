.PHONY: build bump-version help clean

help:
	@echo "Available commands:"
	@echo "  make build         - Build binaries for all platforms"
	@echo "  make bump-version  - Create a new version bump and tag"
	@echo "  make clean         - Remove build directory"

build:
	@echo "🔨 Building for multiple platforms..."
	@mkdir -p build
	@echo "  → Linux (amd64)..."
	@GOOS=linux GOARCH=amd64 go build -o build/ssh-tunnel-manager-linux-amd64
	@echo "  → macOS (Intel)..."
	@GOOS=darwin GOARCH=amd64 go build -o build/ssh-tunnel-manager-darwin-amd64
	@echo "  → macOS (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 go build -o build/ssh-tunnel-manager-darwin-arm64
	@echo "✅ Build complete! Binaries in ./build/"

clean:
	@rm -rf build
	@echo "✅ Build directory cleaned"

bump-version:
	@echo "╭─────────────────────────────────────────────╮"
	@echo "│         VERSION BUMP ASSISTANT              │"
	@echo "╰─────────────────────────────────────────────╯"
	@echo ""
	@echo "Current version: $$(grep 'const Version' version.go | cut -d'"' -f2)"
	@echo ""
	@read -p "Enter new version (e.g., 0.2.0): " VERSION; \
	if [ -z "$$VERSION" ]; then \
		echo "❌ Version cannot be empty"; \
		exit 1; \
	fi; \
	echo ""; \
	echo "Enter changes (one per line, press Enter twice to finish):"; \
	CHANGES=""; \
	while IFS= read -r line; do \
		[ -z "$$line" ] && break; \
		CHANGES="$$CHANGES\n- $$line"; \
	done; \
	if [ -z "$$CHANGES" ]; then \
		echo "❌ No changes provided"; \
		exit 1; \
	fi; \
	echo ""; \
	echo "📝 Updating version to $$VERSION..."; \
	sed -i "s/const Version = \".*\"/const Version = \"$$VERSION\"/" version.go; \
	echo ""; \
	echo "📝 Updating CHANGELOG.md..."; \
	DATE=$$(date +%Y-%m-%d); \
	TEMP=$$(mktemp); \
	echo "# Changelog\n" > $$TEMP; \
	echo "All notable changes to this project will be documented in this file.\n" >> $$TEMP; \
	echo "## [$$VERSION] - $$DATE\n" >> $$TEMP; \
	echo "### Added" >> $$TEMP; \
	echo "$$CHANGES" >> $$TEMP; \
	echo "" >> $$TEMP; \
	tail -n +4 CHANGELOG.md >> $$TEMP; \
	mv $$TEMP CHANGELOG.md; \
	echo ""; \
	echo "📦 Committing changes..."; \
	git add version.go CHANGELOG.md; \
	git commit -m "Bump version to $$VERSION"; \
	echo ""; \
	echo "🏷️  Creating tag v$$VERSION..."; \
	git tag -a "v$$VERSION" -m "Release v$$VERSION"; \
	echo ""; \
	echo "✅ Version bumped to $$VERSION"; \
	echo "✅ Changes added to CHANGELOG.md"; \
	echo "✅ Tag v$$VERSION created"; \
	echo ""; \
	echo "To push changes, run:"; \
	echo "  git push && git push --tags"
