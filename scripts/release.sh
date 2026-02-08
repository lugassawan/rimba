#!/bin/sh
set -eu

usage() {
    echo "Usage: $0 <version>"
    echo ""
    echo "Create and push a release tag to trigger the GitHub Actions release workflow."
    echo ""
    echo "Examples:"
    echo "  $0 0.1.0"
    echo "  $0 v0.1.0"
    exit 1
}

die() {
    echo "Error: $1" >&2
    exit 1
}

# Require a version argument
[ $# -eq 1 ] || usage

version="$1"

# Normalize: ensure v prefix
case "$version" in
    v*) ;;
    *)  version="v${version}" ;;
esac

# Validate semver format (vX.Y.Z)
echo "$version" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$' \
    || die "invalid version format: $version (expected vX.Y.Z)"

# Ensure we're on main
branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$branch" != "main" ]; then
    echo "Not on main (currently on $branch). Switching to main..."
    git checkout main
    git pull origin main
fi

# Ensure working tree is clean
if [ -n "$(git status --porcelain)" ]; then
    die "working tree is dirty — commit or stash changes first"
fi

# Ensure tag doesn't already exist
if git rev-parse "$version" >/dev/null 2>&1; then
    die "tag $version already exists"
fi

echo "Creating tag $version..."
git tag "$version"

echo "Pushing tag $version to origin..."
git push origin "$version"

echo ""
echo "Released $version — GitHub Actions will build and publish the release."
