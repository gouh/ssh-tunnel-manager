#!/bin/bash

set -e

VERSION_FILE="version.go"
CHANGELOG_FILE="CHANGELOG.md"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_header() {
    echo -e "${YELLOW}╭─────────────────────────────────────────────╮"
    echo "│         VERSION BUMP ASSISTANT              │"
    echo "╰─────────────────────────────────────────────╯${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

get_current_version() {
    grep 'const Version' "$VERSION_FILE" | cut -d'"' -f2
}

validate_semver() {
    local version="$1"
    if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        return 1
    fi
    return 0
}

prompt_version() {
    local current=$(get_current_version)
    echo "Current version: $current"
    echo ""
    
    while true; do
        read -p "Enter new version (e.g., 1.2.0): " VERSION
        if [ -z "$VERSION" ]; then
            print_error "Version cannot be empty"
            continue
        fi
        if ! validate_semver "$VERSION"; then
            print_error "Invalid semver format. Use MAJOR.MINOR.PATCH (e.g., 1.2.0)"
            continue
        fi
        break
    done
}

prompt_type() {
    echo ""
    echo "Select change type:"
    echo "  1) Features      (feat)"
    echo "  2) Bug fixes     (fix)"
    echo "  3) Documentation (docs)"
    echo "  4) Refactoring   (refactor)"
    echo "  5) Other         (chore)"
    echo ""
    
    while true; do
        read -p "Enter type (1-5) [1]: " TYPE_CHOICE
        TYPE_CHOICE=${TYPE_CHOICE:-1}
        
        case $TYPE_CHOICE in
            1) TYPE="feat"; break ;;
            2) TYPE="fix"; break ;;
            3) TYPE="docs"; break ;;
            4) TYPE="refactor"; break ;;
            5) TYPE="chore"; break ;;
            *) print_error "Invalid option. Choose 1-5" ;;
        esac
    done
}

prompt_changes() {
    echo ""
    echo "Enter changes (one per line, press Enter twice to finish):"
    echo ""
    
    local changes=()
    while IFS= read -r line; do
        [ -z "$line" ] && break
        changes+=("$line")
    done
    
    if [ ${#changes[@]} -eq 0 ]; then
        print_error "No changes provided"
        exit 1
    fi
    
    CHANGES=$(printf '%s\n' "${changes[@]}")
}

update_version() {
    echo ""
    echo "📝 Updating version to $VERSION..."
    sed -i "s/const Version = \".*\"/const Version = \"$VERSION\"/" "$VERSION_FILE"
    print_success "Version updated in $VERSION_FILE"
}

update_changelog() {
    echo ""
    echo "📝 Updating CHANGELOG.md..."
    
    local date=$(date +%Y-%m-%d)
    local temp=$(mktemp)
    
    {
        echo "# Changelog"
        echo ""
        echo "All notable changes to this project will be documented in this file."
        echo ""
        echo "## [$VERSION] - $date"
        echo ""
        echo "### $TYPE"
        echo ""
        while IFS= read -r line; do
            echo "- $line"
        done <<< "$CHANGES"
        echo ""
        tail -n +4 "$CHANGELOG_FILE"
    } > "$temp"
    
    mv "$temp" "$CHANGELOG_FILE"
    print_success "CHANGELOG.md updated"
}

commit_changes() {
    echo ""
    echo "📦 Committing changes..."
    git add "$VERSION_FILE" "$CHANGELOG_FILE"
    git commit -m "Bump version to $VERSION"
    print_success "Commit created"
}

create_tag() {
    echo ""
    echo "🏷️  Creating tag v$VERSION..."
    git tag -a "v$VERSION" -m "Release v$VERSION"
    print_success "Tag v$VERSION created"
}

main() {
    print_header
    
    prompt_version
    prompt_type
    prompt_changes
    
    echo ""
    echo "─────────────────────────────────────────────"
    echo "  Version: $VERSION"
    echo "  Type:    $TYPE"
    echo "  Changes: $(echo "$CHANGES" | wc -l) item(s)"
    echo "─────────────────────────────────────────────"
    echo ""
    
    read -p "Proceed? [Y/n] " confirm
    if [[ "$confirm" =~ ^[Nn]$ ]]; then
        echo "Aborted."
        exit 0
    fi
    
    update_version
    update_changelog
    commit_changes
    create_tag
    
    echo ""
    echo "─────────────────────────────────────────────"
    print_success "Version bumped to $VERSION"
    echo "─────────────────────────────────────────────"
    echo ""
    echo "To push changes, run:"
    echo "  git push && git push --tags"
}

main "$@"
