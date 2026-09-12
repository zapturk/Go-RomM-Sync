#!/usr/bin/env bash
set -eo pipefail

# ANSI color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

echo -e "${BLUE}=== Go-RomM-Sync Pre-Push Verification ===${NC}\n"

FAILURES=0

# 1. Formatting check (gofmt)
echo -e "${YELLOW}[1/7] Checking code formatting (gofmt)...${NC}"
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo -e "${RED}Found unformatted Go files:${NC}"
    echo "$UNFORMATTED"
    echo -e "${YELLOW}Auto-formatting files with gofmt -w...${NC}"
    # shellcheck disable=SC2086
    gofmt -w $UNFORMATTED
    echo -e "${YELLOW}Please review and stage the newly formatted files.${NC}"
    FAILURES=$((FAILURES + 1))
else
    echo -e "${GREEN}✓ All Go files are properly formatted.${NC}"
fi

# 2. Go vet
echo -e "\n${YELLOW}[2/7] Running go vet...${NC}"
if go vet ./...; then
    echo -e "${GREEN}✓ go vet passed.${NC}"
else
    echo -e "${RED}✗ go vet found issues.${NC}"
    FAILURES=$((FAILURES + 1))
fi

# 3. Cognitive complexity (gocognit -over 30)
echo -e "\n${YELLOW}[3/7] Checking cognitive complexity (gocognit <= 30)...${NC}"
if go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 30 .; then
    echo -e "${GREEN}✓ Cognitive complexity check passed (no functions > 30).${NC}"
else
    echo -e "${RED}✗ Found functions exceeding cognitive complexity threshold of 30.${NC}"
    FAILURES=$((FAILURES + 1))
fi

# 4. Go-critic / static checks
echo -e "\n${YELLOW}[4/7] Running gocritic checks...${NC}"
if go run github.com/go-critic/go-critic/cmd/gocritic@latest check -enableAll -disable=whyNoLint ./...; then
    echo -e "${GREEN}✓ gocritic passed.${NC}"
else
    echo -e "${RED}✗ gocritic found issues.${NC}"
    FAILURES=$((FAILURES + 1))
fi

# 5. Vulnerability check (govulncheck)
echo -e "\n${YELLOW}[5/7] Running vulnerability check (govulncheck)...${NC}"
if go run golang.org/x/vuln/cmd/govulncheck@latest ./...; then
    echo -e "${GREEN}✓ Vulnerability check passed.${NC}"
else
    echo -e "${RED}✗ govulncheck found vulnerabilities.${NC}"
    FAILURES=$((FAILURES + 1))
fi

# 6. Go unit tests
echo -e "\n${YELLOW}[6/7] Running unit tests (go test ./...)...${NC}"
if go test ./...; then
    echo -e "${GREEN}✓ All Go tests passed.${NC}"
else
    echo -e "${RED}✗ Go unit tests failed.${NC}"
    FAILURES=$((FAILURES + 1))
fi

# 7. Frontend build (if frontend exists)
if [ -d "frontend" ]; then
    echo -e "\n${YELLOW}[7/7] Verifying frontend build...${NC}"
    if npm --prefix frontend run build; then
        echo -e "${GREEN}✓ Frontend build succeeded.${NC}"
    else
        echo -e "${RED}✗ Frontend build failed.${NC}"
        FAILURES=$((FAILURES + 1))
    fi
else
    echo -e "\n${YELLOW}[7/7] Frontend directory not found, skipping.${NC}"
fi

echo -e "\n=========================================="
if [ $FAILURES -eq 0 ]; then
    echo -e "${GREEN}✓ ALL PRE-PUSH CHECKS PASSED! Ready to push.${NC}"
    exit 0
else
    echo -e "${RED}✗ $FAILURES CHECK(S) FAILED. Fix the issues before pushing.${NC}"
    exit 1
fi
