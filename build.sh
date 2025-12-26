#!/bin/bash
echo "🚀 Building Orchix..."

# Build with version info
VERSION="v0.2.0"
BUILD_DATE=$(date +'%Y-%m-%d %H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "dev")

echo "Version: $VERSION"
echo "Build Date: $BUILD_DATE"
echo "Git Commit: $GIT_COMMIT"

# Build the binary
go build -ldflags="-X 'main.Version=$VERSION' -X 'main.BuildDate=$BUILD_DATE' -X 'main.GitCommit=$GIT_COMMIT'" \
    -o orchix ./cmd/Orchix

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
    chmod +x orchix
    echo "Binary: ./orchix"
    echo ""
    echo "Test it:"
    echo "  ./orchix version"
    echo "  ./orchix --help"
else
    echo "❌ Build failed"
    exit 1
fi