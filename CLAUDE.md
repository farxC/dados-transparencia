# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Service Does

ETL + REST API that crawls the Brazilian Transparency Portal (Portal da Transparência), downloads ZIP files containing CSV data, filters by IFRO management unit codes, transforms the data, and persists only the relevant records. Exposes the stored data via a REST API consumed by `SAGA_IFRO_API`.

## Commands

```bash
# API server
go run cmd/api/main.go

# ETL (all flags shown with defaults)
go run cmd/etl/main.go \
  -kind=expenses_execution \   # or: expenses
  -init=2025-01-01 \
  -end=2025-01-31 \
  -codes='158454,158148' \     # comma-separated unit/management codes
  -byManagingCode=false \      # true = filter by management code column
  -concurrency=10 \
  -loglevel=info \             # debug, info, warn, error
  -trigger=MANUAL \            # MANUAL or SCHEDULED
  -debug=false                 # true = save filtered CSVs, bypass history checks

# Migrations
make migrate-up
make migrate-down N
make migration NAME            # create new migration pair

# Swagger docs regeneration
make gen-docs

# Tests
go test ./...
go test ./internal/utils/...   # run a specific package
```

## Architecture

### DDD Layers

```
cmd/api/          → Chi HTTP server (routes, handlers, middleware)
cmd/etl/          → CLI orchestrator entry point
cmd/migrate/      → Migration runner + SQL files

internal/domain/
  model/          → Entities (pure structs, no DB tags)
  repository/     → Repository interfaces
  service/        → DTOs, gateway/loader interfaces, assembler logic

internal/application/
  orchestrator.go                      → Generic Orchestrator[J] with worker pool, retries, idempotency
  pipeline_expenses_daily.go           → Pipeline[ExpensesDailyJob]
  pipeline_expenses_execution.go       → Pipeline[ExpensesExecutionJob]

internal/infrastructure/
  client/portal/  → HTTP downloader (portal.go), DataFrame queries (query.go), CSV mappers (mapper.go)
  store/          → sqlx repository implementations + transactional loader
  filesystem/     → ZIP extraction, CSV reading (encoding-aware)
  db/             → sqlx connection pool
  env/            → GetString/GetInt env helpers
  logger/         → Leveled structured logger

internal/utils/
  parser.go       → ParseFloat (pt-BR format), ParseDate, ParseInt64, ParseBool
```

### Two Extraction Types

| Kind | Granularity | Source | Data |
|------|-------------|--------|------|
| `expenses` | Per day | `despesas-YYYYMMDD.zip` | Commitment → Liquidation → Payment full lifecycle |
| `expenses_execution` | Per month | `despesas-execucao/YYYYMM.zip` | Monthly aggregated budget execution rows |

### ETL Pipeline Steps (both kinds follow the same contract)

1. **Download** — `portal.go` fetches ZIP from transparency portal (User-Agent spoofed)
2. **Extract** — `filesystem/local.go` unzips to `tmp/data/`; skips irrelevant files (Bancos, Faturas, Precatorios)
3. **Filter** — `client/portal/query.go` uses Gota DataFrames to match rows by `Código Gestão` or `Código Unidade Gestora`
4. **Map** — `client/portal/mapper.go` converts raw CSV rows to domain entities; parsers handle pt-BR float format (e.g. `1.234,56`)
5. **Assemble** — `domain/service/assembler.go` groups flat entities into hierarchies (items under commitments, impacts under payments)
6. **Load** — `infrastructure/store/loader.go` runs per-unit DB transactions with upserts (`ON CONFLICT DO UPDATE`) and orphan cleanup (DELETE children, then INSERT)

### Orchestrator Pattern

`Orchestrator[J any]` in `internal/application/orchestrator.go`:
- Worker pool (configurable concurrency, default 10)
- Loads `IngestionHistory` at startup to skip already-processed jobs (`ShouldProcess`)
- Creates `IN_PROGRESS` record before processing; updates to `SUCCESS`/`FAILURE`/`SKIPPED` after
- Up to 3 retries per job; stale-timeout at 30 minutes
- Extending to a new extraction type: implement the `Pipeline[J]` interface (5 methods: `Execute`, `BuildHistoryRecord`, `ShouldSkip`, `StatusKey`, `HistoryRange`)

### Domain Model Key Entities

- **Commitment** (`empenho`) — has `[]CommitmentItem`, each with `[]CommitmentItemsHistory`
- **Liquidation** — has `[]LiquidationImpactedCommitment` (links to commitment codes)
- **Payment** — has `[]PaymentImpactedCommitment`
- **ExpenseExecution** — monthly aggregate (committed/liquidated/paid values per budget line)
- **IngestionHistory** — audit record per job (status, codes processed, trigger type)

### API Endpoints

Base: `/v1/`

| Method | Path | Handler |
|--------|------|---------|
| GET | `/expenses/summary` | Summary by management unit |
| GET | `/expenses/summary/by-management` | Global summary |
| GET | `/expenses/budget-execution/report` | Budget report by expense nature |
| GET | `/expenses/top-favored` | Top suppliers by payment value |
| GET | `/budget-execution/` | Monthly execution data |
| GET | `/commitments/` | Commitments with items & history |
| GET | `/ingestion/history` | Audit records |
| POST | `/ingestion` | Create ingestion record |
| GET | `/health` | Health check |

Most queries require `management_code`; optionally accept `management_unit_codes`, `start_date`, `end_date` (YYYY-MM-DD).

## Configuration

Key environment variables (loaded from `.env` by Makefile, read at runtime via `internal/infrastructure/env/`):

```
DB_ADDR               # full postgres connection string
DB_MAX_OPEN_CONNS     # default 25
DB_MAX_IDLE_CONNS     # default 25
DB_MAX_IDLE_TIME      # default 15m
ADDR                  # API listen address (default :8080)
```

## Testing

Only `internal/utils/parser_test.go` exists — table-driven tests for pt-BR number/date parsing. There are no DB integration tests; infrastructure is tested manually via Docker.

```bash
go test ./internal/utils/
```

## Important Quirks

- **pt-BR float format**: `utils/parser.go:ParseFloat` handles both `1.234,56` and `1234.56` — always use this, never `strconv.ParseFloat` directly on portal data.
- **Idempotency relies on**: (a) unique DB constraints on `commitment_code`, `payment_code`, `liquidation_code`; (b) orchestrator status map from `IngestionHistory`. The ETL is safe to re-run.
- **`-debug=true`**: saves filtered DataFrames to CSV and bypasses `IngestionHistory` checks — useful for investigating raw portal data without polluting the history table.
- **Tmp dirs**: ETL creates `tmp/zips/` and `tmp/data/` under the working directory at startup. Already-downloaded ZIPs are reused (checked via `os.Stat`).
- **`expenses` vs `expenses_execution`** are entirely separate data sources with separate DB tables and separate pipelines — don't conflate them.
