#!/bin/bash
# validate-app.sh — Validate a generated nanowave app project
# Usage: ./scripts/validate-app.sh <project-dir> <app-name>
# Returns exit code 0 (pass) or 1 (violations found)

set -euo pipefail

PROJECT_DIR="${1:?Usage: validate-app.sh <project-dir> <app-name>}"
APP_NAME="${2:?Usage: validate-app.sh <project-dir> <app-name>}"

VIOLATIONS=0
PASS_COUNT=0

# Color output (if terminal supports it)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    echo -e "  ${GREEN}PASS${NC} $1"
}

fail() {
    VIOLATIONS=$((VIOLATIONS + 1))
    echo -e "  ${RED}FAIL${NC} $1"
}

warn() {
    echo -e "  ${YELLOW}WARN${NC} $1"
}

echo "=== Nanowave App Validation ==="
echo "Project: $PROJECT_DIR"
echo "App: $APP_NAME"
echo ""

# --- Check project directory exists ---
if [ ! -d "$PROJECT_DIR" ]; then
    echo "ERROR: Project directory does not exist: $PROJECT_DIR"
    exit 1
fi

# --- File Structure ---
echo "--- File Structure ---"

if [ -f "$PROJECT_DIR/${APP_NAME}.xcodeproj/project.pbxproj" ] || [ -f "$PROJECT_DIR/project.yml" ]; then
    pass "Xcode project exists"
else
    fail "No Xcode project found (expected ${APP_NAME}.xcodeproj or project.yml)"
fi

# Check for AppTheme.swift
APPTHEME_FILES=$(find "$PROJECT_DIR" -name "AppTheme.swift" -not -path "*/.build/*" 2>/dev/null || true)
if [ -n "$APPTHEME_FILES" ]; then
    pass "AppTheme.swift exists"
else
    fail "AppTheme.swift not found — design tokens are required"
fi

# Check for @main App entry point
MAIN_APP_FILES=$(grep -rl '@main' "$PROJECT_DIR" --include="*.swift" 2>/dev/null || true)
if [ -n "$MAIN_APP_FILES" ]; then
    pass "@main App entry point found"
else
    fail "No @main App entry point found"
fi

echo ""

# --- AppTheme Compliance ---
echo "--- AppTheme Compliance ---"

# Check for hardcoded Color literals (excluding AppTheme.swift itself)
HARDCODED_COLORS=$(grep -rn 'Color(\s*\.' "$PROJECT_DIR" --include="*.swift" 2>/dev/null | grep -v "AppTheme" | grep -v "Assets" | grep -v "//.*Color" || true)
if [ -n "$HARDCODED_COLORS" ]; then
    fail "Hardcoded Color literals found (should use AppTheme.Colors):"
    echo "$HARDCODED_COLORS" | head -10 | while IFS= read -r line; do
        echo "    $line"
    done
    COUNT=$(echo "$HARDCODED_COLORS" | wc -l | tr -d ' ')
    if [ "$COUNT" -gt 10 ]; then
        echo "    ... and $((COUNT - 10)) more"
    fi
else
    pass "No hardcoded Color literals"
fi

# Check for hardcoded .font(.system(...))
HARDCODED_FONTS=$(grep -rn '\.font(\.system' "$PROJECT_DIR" --include="*.swift" 2>/dev/null | grep -v "AppTheme" | grep -v "//.*font" || true)
if [ -n "$HARDCODED_FONTS" ]; then
    fail "Hardcoded font specifications found (should use AppTheme.Fonts):"
    echo "$HARDCODED_FONTS" | head -5 | while IFS= read -r line; do
        echo "    $line"
    done
else
    pass "No hardcoded font specifications"
fi

echo ""

# --- MVVM Architecture ---
echo "--- MVVM Architecture ---"

# Check ViewModels have @Observable
VIEWMODEL_FILES=$(grep -rl 'ViewModel' "$PROJECT_DIR" --include="*.swift" 2>/dev/null | grep -v "_test\|Test\|Preview" || true)
MISSING_OBSERVABLE=0
if [ -n "$VIEWMODEL_FILES" ]; then
    while IFS= read -r file; do
        if grep -q 'class.*ViewModel' "$file" 2>/dev/null; then
            if ! grep -q '@Observable' "$file" 2>/dev/null; then
                fail "ViewModel missing @Observable: $file"
                MISSING_OBSERVABLE=$((MISSING_OBSERVABLE + 1))
            fi
        fi
    done <<< "$VIEWMODEL_FILES"
    if [ "$MISSING_OBSERVABLE" -eq 0 ]; then
        pass "All ViewModels have @Observable"
    fi
else
    warn "No ViewModel files found"
fi

# Check Views have #Preview
VIEW_FILES=$(find "$PROJECT_DIR" -name "*View.swift" -not -path "*/.build/*" -not -name "AppTheme*" 2>/dev/null || true)
MISSING_PREVIEW=0
if [ -n "$VIEW_FILES" ]; then
    while IFS= read -r file; do
        if ! grep -q '#Preview' "$file" 2>/dev/null; then
            fail "View missing #Preview: $file"
            MISSING_PREVIEW=$((MISSING_PREVIEW + 1))
        fi
    done <<< "$VIEW_FILES"
    if [ "$MISSING_PREVIEW" -eq 0 ]; then
        pass "All Views have #Preview blocks"
    fi
fi

echo ""

# --- Forbidden Patterns ---
echo "--- Forbidden Patterns ---"

# Check for URLSession / networking
NETWORKING=$(grep -rn 'URLSession\|URLRequest\|Alamofire\|AF\.' "$PROJECT_DIR" --include="*.swift" 2>/dev/null | grep -v "//.*URL" || true)
if [ -n "$NETWORKING" ]; then
    fail "Networking code found (apps should be offline-only):"
    echo "$NETWORKING" | head -5 | while IFS= read -r line; do
        echo "    $line"
    done
else
    pass "No networking code"
fi

# Check for CoreData
COREDATA=$(grep -rn 'import CoreData\|NSManagedObject\|NSPersistentContainer' "$PROJECT_DIR" --include="*.swift" 2>/dev/null || true)
if [ -n "$COREDATA" ]; then
    fail "CoreData usage found (use SwiftData instead):"
    echo "$COREDATA" | head -5 | while IFS= read -r line; do
        echo "    $line"
    done
else
    pass "No CoreData usage"
fi

echo ""

# --- Summary ---
echo "=== Summary ==="
echo "Checks passed: $PASS_COUNT"
echo "Violations: $VIOLATIONS"

if [ "$VIOLATIONS" -gt 0 ]; then
    echo -e "${RED}RESULT: FAIL${NC}"
    exit 1
else
    echo -e "${GREEN}RESULT: PASS${NC}"
    exit 0
fi
