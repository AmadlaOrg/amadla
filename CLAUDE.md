# amadla — Thin Orchestrator CLI

## AI Skills

Follow the practices defined in `~/Projects/SiteNetSoft/ai-skills/`:
- `dev-practices/golang/` — Go style, error handling, functions, testing, linting
- `dev-practices/git/` — Git authorship rules, multi-repo workspace patterns

## Overview

`amadla` is the top-level orchestrator for the Amadla tool pipeline. It reads `.hery` entity files, builds a dependency graph from `_requires`, topologically sorts them, and shells out to registered tools in order.

## Architecture

```
main.go          → Root Cobra command with --config flag
cmd/
  run.go         → Core pipeline: load config → read .hery → build DAG → sort → exec tools
  init.go        → Bootstrap tools.hery by scanning PATH for standard Amadla tools
  list.go        → Tabular display of registered tools and their status
  doctor.go      → Verify tool installation and compatibility
dag/
  dag.go         → Topological sort with cycle detection (DFS)
toolconfig/
  toolconfig.go  → Parse tools.hery, resolve tool binaries via PATH
```

## How `run` works

1. Loads `~/.config/amadla/tools.hery` (or `--config` path)
2. Builds `entity_type → tool` lookup from config
3. Reads `.hery` files from target directory, extracts `_type` and `_requires`
4. Builds DAG, topologically sorts
5. For each entity in order: marshals to JSON, pipes to tool's stdin, sets `AMADLA_ENTITY` env var
6. `--dry-run` prints execution order without running

## Tool Pipeline

The standard execution pipeline is: `raise` -> `lay` -> `enjoin` -> `weaver` -> `waiter`

- **raise** provisions infrastructure (VMs, cloud instances)
- **lay** handles installation only: Package, Application, ProgrammingLanguage entities
- **enjoin** handles system state configuration: User, Service, Cron, System/*, Security/* entities
- **weaver** generates configuration files from templates
- **waiter** handles deployment strategies

## Tool discovery

Config-driven via `tools.hery` (entity type `amadla.org/entity/tools@v1.0.0`). Tools are resolved by explicit path or PATH lookup. `amadla init` bootstraps by scanning PATH for the standard tool names (raise, lay, enjoin, weaver, doorman, judge, unravel, waiter, conduct, lighthouse, garbage, dryrun, hery).

## Testing

```bash
make test          # Run all tests
go test ./...      # Same, without coverage
```

Function variables (`osStat`, `execLookPath`, etc.) in `toolconfig/` follow the project-wide DI pattern for test mocking.
