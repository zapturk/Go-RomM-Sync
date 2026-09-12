---
name: pre-push
description: >-
  Runs pre-push verification checks (formatting, gocognit, gocritic, go vet, vulnerability scan, tests, and frontend build) before pushing or submitting code. Use when the user asks to verify, lint, check, or test changes before git push, or asks to run pre-push validation.
---

# Pre-Push Verification Skill

This skill ensures that all codebase formatting, cognitive complexity thresholds, linters, vulnerability scans, unit tests, and frontend builds pass cleanly before committing or pushing changes to the remote repository.

## Quick Run

Run the automated verification script from the repository root:

```bash
./.agents/skills/pre-push/scripts/verify.sh
```

---

## Detailed Step-by-Step Workflow

When executing pre-push checks manually or diagnosing specific failures, run each check in sequence:

### 1. Code Formatting (`gofmt`)
Ensure all Go source files adhere to canonical `gofmt` style (no multiple blank lines, properly indented blocks).

- **Check unformatted files**:
  ```bash
  gofmt -l .
  ```
- **Auto-format any modified files**:
  ```bash
  gofmt -w .
  ```

### 2. Static Analysis (`go vet`)
Verify standard Go semantics, unused variables, and suspicious constructs:

```bash
go vet ./...
```

### 3. Cognitive Complexity (`gocognit`)
CI enforces a maximum cognitive complexity limit of **30** (via `.golangci.yml`):

```bash
go run github.com/uudashr/gocognit/cmd/gocognit@latest -over 30 .
```

- **If a function fails (> 30)**:
  - Break down large functions by extracting distinct sub-tasks (progress reporting, permission checks, archive parsing, cleanup) into private helper functions.
  - Simplify deeply nested conditional branches (`if`/`else`, nested `for` loops).

### 4. Linters & Style (`gocritic`)
Run the linters configured in the repository (diagnostic, style, performance, experimental, opinionated):

```bash
go run github.com/go-critic/go-critic/cmd/gocritic@latest check -enableAll -disable=whyNoLint ./...
```

- **Common Pitfall (`filepathJoin`)**:
  - Do not pass string literals containing path separators (`/` or `\`) to `filepath.Join`.
  - Store directory paths in a variable before calling `filepath.Join(dir, child)` or join path elements individually.

### 5. Vulnerability Scan (`govulncheck`)
Verify that dependencies do not contain known security vulnerabilities (matches CI `build.yml` step):

```bash
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

### 6. Unit Tests (`go test`)
Run the test suite across all packages:

```bash
go test ./...
```

### 7. Frontend Build & Typecheck
If any files under `frontend/` have changed, verify TypeScript types and production assets:

```bash
npm --prefix frontend run build
```

---

## Optional: Git Pre-Push Hook Setup

To automatically run this verification before every `git push`:

```bash
ln -sf "../../.agents/skills/pre-push/scripts/verify.sh" .git/hooks/pre-push
```
