---
type: "always_apply"
---

# Project context
- Project: Multi-User Personal Inventory App/Service (backend in Go, database PostgreSQL, frontend React/TypeScript).
- Development phase: active, no need for backwards compatibility, breaking changes are acceptable.

# Development Environment and Command Execution
- I run on Windows using PowerShell. When executing shell commands, use PowerShell syntax and avoid bash-specific operators like `&&` which fail in PowerShell with the error "The token '&&' is not a valid statement separator in this version." Use semicolons (`;`) or separate commands instead.

# Go Testing Standards
- For Go unit tests, always use frankban's quicktest testing framework with the import alias `qt` (import "github.com/frankban/quicktest" as qt).
- Structure tests as table-driven tests, but maintain separate test functions or clearly separated test cases for happy path and error scenarios to avoid conditional logic (if/else statements) within test cases.
- In table-driven tests, use `t.Run()` for subtests (not `c.Run()`), and within each subtest create a new quicktest instance with `c := qt.New(t)`.
- Use `qt.IsNotNil(value)` instead of `qt.Not(qt.IsNil(value))` for nil checks.
- Always write tests in separate `*_test.go` files using the `_test` package suffix (e.g., `package mypackage_test`) to ensure testing only public interfaces and maintaining proper encapsulation.

# Code Documentation and Language
- Write all code comments, documentation, and godoc comments in English, regardless of the language used in our conversation.
- Write comprehensive godoc comments for all public interfaces (exported functions, types, methods, constants, and variables) following Go documentation conventions.

# Development Workflow
- When responses become too long or complex, break down tasks into smaller, manageable parts and execute them incrementally.
- Maintain consistency in coding style, naming conventions, and architectural patterns throughout the codebase.
- For any newly written Go code, proactively suggest writing corresponding unit tests when it makes logical sense (skip trivial getters/setters or simple data structures).

# Git Operations
- Always use the MCP git tools (`git_status`, `git_add`, `git_commit`, `git_push`, `git_pull`, `git_diff`, `git_diff_staged`, `git_diff_unstaged`, `git_log`, `git_show`, `git_checkout`, `git_create_branch`, `git_apply_patch_string`, etc.) for repository operations. Do NOT run `git` through `launch-process` for these.
- Only fall back to `launch-process` with raw `git` when the operation cannot be expressed via the MCP tools (e.g., `git stash`, `git fetch`, interactive rebase, complex pipelines), and call out the fallback explicitly.

# Additional information
Refer to .github/copilot-instructions.md and CLAUDE.md for additional instructions
