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
	@chmod +x scripts/bump-version.sh
	@./scripts/bump-version.sh
