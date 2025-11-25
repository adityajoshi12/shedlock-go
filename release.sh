#!/bin/bash
set -e

# ShedLock-Go Release Script

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Usage: ./release.sh v1.0.0"
    echo ""
    echo "This script will:"
    echo "  1. Run tests"
    echo "  2. Update documentation"
    echo "  3. Create and push git tag"
    echo "  4. Trigger GitHub release"
    exit 1
fi

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format v1.0.0"
    exit 1
fi

echo "🚀 Releasing ShedLock-Go $VERSION"
echo ""

# Check if git is clean
if [[ -n $(git status -s) ]]; then
    echo "⚠️  Warning: You have uncommitted changes"
    echo ""
    git status -s
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Run tests
echo "🧪 Running tests..."
go test -v $(go list ./... | grep -v /examples/) || {
    echo "❌ Tests failed"
    exit 1
}
echo "✅ Tests passed"
echo ""

# Run linter (if available)
if command -v golangci-lint &> /dev/null; then
    echo "🔍 Running linter..."
    golangci-lint run ./... || {
        echo "❌ Linter failed"
        exit 1
    }
    echo "✅ Linter passed"
    echo ""
fi

# Build examples
echo "🔨 Building examples..."
go build -v ./examples/postgres || exit 1
go build -v ./examples/redis || exit 1
go build -v ./examples/scheduler || exit 1
echo "✅ Examples built successfully"
echo ""

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "❌ Tag $VERSION already exists"
    exit 1
fi

# Create tag
echo "🏷️  Creating tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"
echo "✅ Tag created"
echo ""

# Push tag
echo "📤 Pushing tag to origin..."
git push origin "$VERSION"
echo "✅ Tag pushed"
echo ""

echo "🎉 Release $VERSION completed!"
echo ""
echo "Next steps:"
echo "  1. Wait a few minutes for pkg.go.dev to index the new version"
echo "  2. Check: https://pkg.go.dev/github.com/adityajoshi12/shedlock-go@$VERSION"
echo "  3. GitHub Release will be created automatically by CI"
echo ""
echo "Users can now install with:"
echo "  go get github.com/adityajoshi12/shedlock-go@$VERSION"

