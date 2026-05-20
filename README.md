# Football League Simulation API

A production-grade Go REST API that simulates a 4-team, double round-robin
mini Premier League over 6 weeks. The simulator uses a Poisson xG model
for match results and a 10,000-run Monte Carlo for championship
probabilities. The whole stack is wired together in `cmd/server/main.go` —
nothing else constructs concrete dependencies.

## Requirements compliance (cursorrules checklist)

Most of the specification is satisfied: Go-only codebase, Chi router, Postgres
via `pgx/v5` and `pgxpool`, raw SQL (no ORM), env-driven `config.Load()`,
transactions for multi-step writes (`TxManager`), read/write repo split,
composition root wiring, mocks under `/mocks`, shared `pkg/response` and
`pkg/validator`, Poisson in `pkg/poisson`, and the full HTTP surface area
below. Automated checks: run `make test`, `make test-race`, and `make vet`.

**Intentional deviations**

- **Docker Compose**: The rules describe a bundled `postgres:15` service with
  `depends_on` and migrations on first boot. This repo targets **hosted
  Postgres on Supabase** instead, so `docker-compose.yml` runs only the `app`
  service and expects `DATABASE_URL` in `.env`. The database is still
  PostgreSQL and the schema is the same; only the topology changed.
- **Migrations at container startup**: Schema is **not** applied automatically
  when the container starts. Apply once per database with `make migrate` (or
  equivalent SQL runner) against your Supabase Session pooler URL.
- **Makefile `make build`**: The compiled binary is written to **`bin/league-api`**
  (not `bin/api` as in the example wording in the rules).

Everything else—including ACID flows for simulate week / edit fixture / play
all, standings recalculation reuse, Monte Carlo predictions from week 4, and
unit tests—is aligned with the task brief.

## Architecture

```
                ┌───────────────────────────────────────────────┐
                │           cmd/server/main.go                │
                │   (composition root — wires everything)       │
                └───────────────────────────────────────────────┘
                                 │ depends on interfaces only
        ┌────────────────────────┼─────────────────────────────────┐
        │                        │                                 │
┌──────────────┐         ┌───────────────┐                ┌────────────────┐
│   handler/   │   uses  │   service/    │     uses       │  repository/   │
│ (HTTP I/O)   │ ─────▶  │ (business)    │ ─────────────▶ │ (Postgres)     │
└──────────────┘         └───────────────┘                └────────────────┘
        │                        │                                 │
        └─────────── all depend on ────────────────▶┌──────────────────────┐
                                                    │     domain/          │
                                                    │ entities + ifaces    │
                                                    └──────────────────────┘
                                                              ▲
                                          ┌───────────────────┴─────────────┐
                                          │ pkg/poisson  pkg/response       │
                                          │ pkg/validator                   │
                                          └─────────────────────────────────┘
```

- `domain` — pure data and interfaces. No I/O, no DB, no HTTP.
- `repository` — Postgres adapters (`pgx/v5`) implementing the domain
  interfaces. Every write accepts a `pgx.Tx`.
- `service` — business logic. SimulateWeek, EditFixture, predictor.
- `handler` — HTTP I/O. Parse → validate → delegate → render.
- `pkg/*` — leaf utilities that nothing else may depend on.

## Prerequisites

- **Go 1.22+**
- **Docker** and **Docker Compose** (optional but recommended for `make docker-up`)
- A **Supabase** (or any PostgreSQL) database reachable from your machine  
  — use the **Session pooler** URI (port **5432**), **`sslmode=require`**.  
  Avoid the Transaction pooler (6543): it breaks prepared statements / locks.
- **`psql`** if you plan to run `make migrate`

## Setup and run

1. Clone and enter the repo directory.

2. Create `.env` from the template and set a real **`DATABASE_URL`**:

   ```bash
   cp .env.example .env
   # Edit .env: paste your Session pooler connection string + password.
   ```

3. **Apply the schema migration once** (creates all tables and seeds the 4 teams):

   ```bash
   make migrate
   ```

4. Start the API using **either** Docker **or** a local Go process:

   ```bash
   # Recommended: build image and run container (reads .env)
   make docker-up
   make docker-logs

   # Or run on the host (also sources .env)
   make run
   ```

5. Open **Swagger UI**: [http://localhost:8080/docs](http://localhost:8080/docs)  
   (adjust host/port if `PORT` in `.env` is not `8080`).

6. Quick CLI check:

   ```bash
   curl -s http://localhost:8080/health
   ```

   Expected JSON shape: `{ "success": true, "data": { "status": "ok" }, "error": null }`  
   (exact field order may vary).

## Environment variables

| Variable       | Required | Description |
|----------------|----------|-------------|
| `DATABASE_URL` | yes      | libpq URI to Postgres (Supabase Session pooler recommended). Must include password and `sslmode=require` when using Supabase. |
| `PORT`         | yes      | TCP port for the HTTP server (e.g. `8080`). |
| `ENV`          | no       | `development` \| `production` \| `test`. Defaults to `development` if unset. |

`DATABASE_URL` has no fallback in code: the process exits if it is missing.

## Migrations

```bash
make migrate    # runs 001_schema.sql then 002_seed.sql via psql
```

Both files are idempotent: `CREATE TABLE IF NOT EXISTS` for the schema,
`ON CONFLICT (name) DO NOTHING` for the team insert.

## API

Every endpoint returns the same envelope:

```json
{ "success": true,  "data": { ... }, "error": null }
{ "success": false, "data": null,    "error": { "code": "...", "message": "..." } }
```

Below, replace `1` with your real **`season id`** returned from creating a season,
and fixture `id` values from **`GET .../fixtures`**.

### `curl` quick reference

| Action | Command |
|--------|---------|
| Health | `curl -s http://localhost:8080/health` |
| Create season | `curl -s -X POST http://localhost:8080/api/seasons` |
| Get season | `curl -s http://localhost:8080/api/seasons/1` |
| Standings | `curl -s http://localhost:8080/api/seasons/1/standings` |
| All fixtures | `curl -s http://localhost:8080/api/seasons/1/fixtures` |
| Week view | `curl -s http://localhost:8080/api/seasons/1/week/3` |
| Simulate next week | `curl -s -X POST http://localhost:8080/api/seasons/1/next-week` |
| Play all remaining | `curl -s -X POST http://localhost:8080/api/seasons/1/play-all` |
| Edit fixture | `curl -s -X PUT http://localhost:8080/api/fixtures/7 -H 'Content-Type: application/json' -d '{"home_goals":3,"away_goals":1}'` |
| Latest predictions | `curl -s http://localhost:8080/api/seasons/1/predictions` |
| Predictions by week | `curl -s http://localhost:8080/api/seasons/1/predictions/4` |
| Reset season | `curl -s -X DELETE http://localhost:8080/api/seasons/1/reset` |

## Swagger UI (interactive API docs)

After `make run` or `make docker-up`, open:

**http://localhost:8080/docs**

That page loads **Swagger UI** (via CDN) against the embedded OpenAPI document served at:

**http://localhost:8080/openapi.yaml**

- Expand an operation → **Try it out** → fill path params / JSON body → **Execute**.
- The spec lives in-repo at `internal/handler/openapi.yaml` if you prefer editing it directly.

Swagger UI loads scripts from **unpkg.com**; allow that domain if your browser blocks third-party scripts.

Suggested flow on a fresh database:

1. **POST** `/api/seasons` — note `data.season.id` and a `data.fixtures[].id` from the response.
2. **POST** `/api/seasons/{id}/next-week` or **POST** `/api/seasons/{id}/play-all`.
3. **GET** `/api/seasons/{id}/standings` or `/predictions`.

If your API listens on another host/port, open `/docs` on that same origin so “Try it out” hits the correct server (`servers.url` is `/`).

## Unit testing

```bash
make test          # all packages
make test-race     # race detector
make vet           # go vet
```

Repository behaviour is exercised through mocks; no real DB required for CI.

## Design decisions

### Why a Poisson xG model?

At its core, the question "who wins?" is reduced to: how many goals will each team score? Whoever scores more, wins. So the real problem is goal prediction, not win prediction.

**Step 1 — Expected Goals (xG)**

xG answers: given the circumstances of this match, how many goals should each team score on average?

- `xG_home = BASE_RATE × (home.Attack / away.Defense) × homeAdvFactor × formFactor`
- `xG_away = BASE_RATE × (away.Attack / home.Defense) × formFactor`
- `BASE_RATE = 1.4`

The base rate of 1.4 comes from real Premier League data — the average team scores approximately 1.4 goals per game. This anchors the entire model to reality.

The **Attack / Defense ratio** captures the strength matchup. If Manchester City (Attack=90) faces Chelsea (Defense=76), the ratio is 90/76 = 1.184 — City is expected to score 18.4% more than the league average against Chelsea's defence. If they face Arsenal (Defense=80) instead, the ratio drops to 1.125. The formula naturally captures relative dominance rather than absolute strength.

The **home advantage factor** is `1.0 + (team.HomeAdvantage / 100)`. Real Premier League data shows home teams win roughly 46% of matches versus 27% for away teams. A HomeAdv of 8 (Arsenal's Emirates) gives a 1.08 multiplier — an 8% boost to xG. Smaller stadiums like Chelsea's Stamford Bridge (HomeAdv=6) give less boost, intentionally grounded in the atmospheric pressure a packed crowd creates.

The **form factor** models momentum. Each of the last 3 results is weighted as Win=1.1, Draw=1.0, Loss=0.9, then averaged. A team on a 3-game winning streak gets a 1.1 multiplier (10% xG boost); a team on 3 losses gets 0.9 (10% reduction). Momentum is a real and measurable phenomenon in football analytics.

**Step 2 — Poisson Distribution**

Once you have xG for each team, you need to convert it into an actual integer goal count. This is where Poisson comes in.

Goals in football are rare, independent events that occur at a roughly constant rate during a match — the textbook definition of a Poisson process. Academic research (Dixon & Coles, 1997 — the foundational paper in football analytics) proved that goals follow a Poisson distribution very closely.

For a given xG (λ), the probability of scoring exactly k goals is: `P(k) = (e^-λ × λ^k) / k!`

With xG = 1.4, this produces: 24.7% chance of 0 goals, 34.5% for 1, 24.2% for 2, 11.3% for 3, 4.0% for 4, and so on. Each team's goals are sampled independently from their own distribution. This means a 0-0 draw is possible even when both xGs are high, a 3-2 thriller can happen between mismatched teams, and upsets occur naturally at statistically realistic rates — without any special-case logic.

**Step 3 — Result Determination**

Once `homeGoals` and `awayGoals` are sampled, the result follows directly: more goals wins (3 points), equal goals draws (1 point each). Goal difference, which is critical for table sorting, falls out of the simulation for free.

**Why this beats simpler approaches**

Most naive approaches either use a random number against a fixed threshold (no scorelines, ignores matchups) or a fixed probability table (rigid, unrealistic distributions). The Poisson xG model produces realistic scorelines from 0-0 to 4-3, generates goal difference automatically, accounts for each specific matchup, incorporates home advantage quantitatively, uses form as a momentum modifier, and is grounded in published football analytics research. The implementation lives behind the `MatchSimulator` interface so a future Elo-, ML-, or shot-volume-based simulator can be swapped in without touching the rest of the codebase.

### How is ACID enforced?

Every multi-step write goes through `domain.TxManager`. The
`repository.PgxTxManager` implementation:

- opens a `pgxpool` transaction with the requested isolation level
  (READ COMMITTED by default, SERIALIZABLE for standings recalculation),
- rolls back if the callback returns an error **or** panics,
- only commits on the happy path.

Repository writers all accept `pgx.Tx` so the same transaction object
threads through every step of `SimulateWeek` and `EditFixture`. Fixture
rows for the week being played are locked with `SELECT … FOR UPDATE`
to prevent concurrent simulations from double-playing the same matches.

### How is SOLID applied?

- **S** — Single responsibility per file: `StandingsService` only does
  standings, `FormService` only does form, handlers only do HTTP I/O.
- **O** — `MatchSimulator`, `ChampionshipPredictor`,
  `StandingsCalculator` and `StandingsSorter` are interfaces that allow
  new strategies without modifying callers.
- **L** — Every repository interface is mocked in `/mocks` and the
  mocks fully substitute for the real thing in service unit tests.
- **I** — Repositories are split into `*Reader` and `*Writer` halves
  so services that only read depend only on the read half.
- **D** — Services accept interfaces, never concrete types.
  `cmd/server/main.go` is the **only** composition root.

### Where is DRY enforced?

- `StandingsService.Calculate` is the single source of truth for the
  league table. Both `SimulateWeek` and `EditFixture` call it.
- `FormService.RecordResult` and `FormMultiplier` centralise form
  handling for the simulator and persistence path.
- `pkg/response` is the only place that constructs response envelopes.
- `pkg/validator` is the only place that validates request input.
- The canonical points / GD / GF ordering lives in the
  `standingsOrderBy` constant in `repository/standings_repo.go`.
- `pkg/poisson` is the only place that samples a Poisson distribution.
- All error codes are constants in `pkg/response/response.go`.
- Team attributes are defined once in `db/migrations/001_schema.sql`
  and inserted at migration time. The API layer always reads from the
  database, so ratings can be updated without touching service or
  handler code.
