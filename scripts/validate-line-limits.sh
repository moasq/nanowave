#!/bin/bash
# validate-line-limits.sh — Check 200-line hard limit for Swift files
# Usage: ./scripts/validate-line-limits.sh <project-dir>
# Returns exit code 0 (pass) or 1 (violations found)

set -euo pipefail

PROJECT_DIR="${1:?Usage: validate-line-limits.sh <project-dir>}"
MAX_LINES=200
VIOLATIONS=0

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo "=== Swift File Line Limit Check ==="
echo "Project: $PROJECT_DIR"
echo "Limit: $MAX_LINES lines"
echo ""

if [ ! -d "$PROJECT_DIR" ]; then
    echo "ERROR: Directory does not exist: $PROJECT_DIR"
    exit 1
fi

SWIFT_FILES=$(find "$PROJECT_DIR" -name "*.swift" -not -path "*/.build/*" -not -path "*/DerivedData/*" 2>/dev/null || true)

if [ -z "$SWIFT_FILES" ]; then
    echo "No Swift files found."
    exit 0
fi

TOTAL=0
while IFS= read -r file; do
    TOTAL=$((TOTAL + 1))
    LINE_COUNT=$(wc -l < "$file" | tr -d ' ')
    if [ "$LINE_COUNT" -gt "$MAX_LINES" ]; then
        VIOLATIONS=$((VIOLATIONS + 1))
        REL_PATH="${file#$PROJECT_DIR/}"
        echo -e "  ${RED}OVER${NC} ${REL_PATH}: ${LINE_COUNT} lines (limit: ${MAX_LINES})"
    fi
done <<< "$SWIFT_FILES"

echo ""
echo "Files checked: $TOTAL"
echo "Over limit: $VIOLATIONS"

if [ "$VIOLATIONS" -gt 0 ]; then
    echo -e "${RED}RESULT: FAIL${NC}"
    exit 1
else
    echo -e "${GREEN}RESULT: PASS${NC}"
    exit 0
fi
