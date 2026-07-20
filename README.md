# Livestock Health Monitor

A single-binary Go web app that simulates virtual animal-health sensors, evaluates readings against threshold rules, raises alerts, and serves a live dashboard — all behind database-backed session auth with Admin/User roles.

## Prerequisites

- **Go 1.22+** (developed against 1.26)
- **Microsoft SQL Server** reachable over TCP (LocalDB, SQL Server Express, or a full instance), with an empty database created for the app
- Windows Authentication (SSPI) **or** a SQL login with rights to create tables in that database

## 1. Create the database

If you don't already have one, create an empty database (name doesn't matter, just match it in `DB_DSN`):

```sql
CREATE DATABASE livestock;
```

## 2. Configure environment variables

All variables are optional — sane defaults are baked in for local development. Set at least `DB_DSN` to point at your SQL Server instance.

| Variable | Default | Notes |
|---|---|---|
| `DB_DSN` | `sqlserver://sa:YourPassword@localhost:1433?database=livestock` | Connection string. Omit the user/password segment entirely to use Windows Integrated Auth (SSPI), e.g. `sqlserver://localhost:1433?database=livestock` |
| `PORT` | `8080` | HTTP listen port |
| `SENSOR_COUNT` | `5` | Number of virtual sensors (animals) to run |
| `COOKIE_SECURE` | `false` | Set to `true` **only** when served over HTTPS — it makes session/CSRF cookies `Secure`, which browsers refuse to send over plain `http://` |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | `admin@livestock.local` / `AdminPass123!` | Seeded admin account (created once, on first run against an empty DB) |
| `DEMO_USER_EMAIL` / `DEMO_USER_PASSWORD` | `user@livestock.local` / `UserPass123!` | Seeded demo user account |

## 3. Run it — one command

From the project root:

```bash
# Windows Auth example (no password needed)
DB_DSN="sqlserver://localhost:1433?database=livestock" go run ./cmd/server

# SQL login example
DB_DSN="sqlserver://sa:YourPassword@localhost:1433?database=livestock" go run ./cmd/server
```

PowerShell equivalent:

```powershell
$env:DB_DSN = "sqlserver://localhost:1433?database=livestock"
go run ./cmd/server
```

On first run against an empty database this will:
1. Auto-migrate all tables (`users`, `sessions`, `animals`, `readings`, `alerts`, `vaccinations`)
2. Seed the admin + demo user accounts above
3. Seed 5 demo animals split between them
4. Start one sensor goroutine per animal and the ingest/alerting worker
5. Start the HTTP server

Then open **http://localhost:8080** — you'll be redirected to `/login`.

## 4. Sign in

| Role | Email | Password | Sees |
|---|---|---|---|
| Admin | `admin@livestock.local` | `AdminPass123!` | All animals, all alerts, user management (`/admin/users`) |
| User | `user@livestock.local` | `UserPass123!` | Only their own animals/alerts |

You can also self-register a new account from the login page's "Create account" tab (always provisioned as `USER`).

## Building a binary instead of `go run`

```bash
go build -o livestock.exe ./cmd/server
./livestock.exe
```

## Running tests

```bash
go test ./...
```

## Project layout

```
cmd/server/           entrypoint: config, DB migrate/seed, wiring, graceful shutdown
internal/models/      shared structs (User, Session, Animal, Reading, Alert, Vaccination)
internal/storage/     GORM/SQL Server data access layer
internal/auth/        password hashing, session tokens, login/logout/register logic
internal/middleware/  session authentication + role-based authorization guards, CSRF
internal/rules/       pure threshold-evaluation logic (unit tested)
internal/sensors/     per-animal goroutines generating simulated readings
internal/api/         REST/JSON handlers (auth, animals, readings, alerts, vaccinations, users)
internal/dashboard/   server-rendered HTML pages + static JS/CSS (embedded into the binary)
```

## Notes

- Sessions are stored in the database (`sessions` table), not JWTs — every authenticated request re-validates against it and refreshes `last_accessed_at`. Expired sessions are rejected on read and swept hourly.
- CSRF is enforced via double-submit cookie (`csrf_token` cookie must be echoed in the `X-CSRF-Token` header on any non-GET request) for all authenticated routes.
- Data is isolated by ownership: a `USER` only ever sees animals/readings/alerts/vaccinations they own; an `ADMIN` sees everything and can manage user accounts/roles.
