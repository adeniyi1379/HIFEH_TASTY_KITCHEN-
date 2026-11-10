# Tatsy POS — Plan

Draft 1.0 (November 2026). Two-terminal restaurant POS for a mini restaurant in Nigeria. Dine-in and takeaway, spoon-based food units, manual payments, local-first with optional cloud sync.

## 1. Core decisions (kept)

- Terminal A is the single writer and source of truth; Terminal B is a LAN client.
- All money as integer kobo; all quantities as integers (spoons, pieces, bottles). No floats.
- Stock ledger is append-only; balances are a cached sum. Corrections are new entries.
- Sales use `BEGIN IMMEDIATE` transactions: stock check and deduction in one transaction, so the last portion can't be oversold.
- Login by system-generated PIN only (6 digits), hashed with argon2. Server-side role checks on every endpoint.
- Roles are strict and non-overlapping:
  - **Cashier** — sells (orders, payments, receipts); the only role mirrored on the customer display.
  - **Manager** — sells and corrects (voids, manual discounts, adjustments); every corrective action lands in the audit log.
  - **Owner** — workers, settings, backups, sync; the audit trail is visible to the owner only. Owners do not sell.
- Paid orders are immutable; voids/refunds create linked reversing records with manager PIN and reason.
- Selling needs only the LAN. Internet (Starlink) is used only by the background sync worker, which is a later, optional phase.

## 2. Architecture

```
Terminal A (seller)                        Terminal B (buyer display)
┌─────────────────────────┐                ┌──────────────────────┐
│  posd.exe (single Go     │   LAN/WiFi     │  Edge/Chrome kiosk   │
│  binary, static IP)      │◄──HTTP/WS─────►│  http://192.168.1.10 │
│  • HTTP API + WebSocket  │  port 8080     │  /display = read-only│
│  • SQLite (WAL)          │                │  mirror of the order │
│  • UI served from binary │                │  (items + total)     │
│  • ESC/POS printing      │                └──────────────────────┘
│  • sync outbox worker    │
└─────────────────────────┘
        │ Starlink (only when available, background)
        ▼
   Cloud copy (later phase, optional)
```

### How the two terminals connect

1. Reserve a fixed IP for Terminal A on the router (e.g. `192.168.1.10`) — do this first.
2. Run one Go binary on A. It serves the UI, the JSON API, WebSocket events, and writes to SQLite. Allow port 8080 through Windows Firewall.
3. Terminal B needs **no install at all** — open Edge/Chrome in kiosk mode to `http://192.168.1.10:8080/display`. It's a read-only buyer view: running order lines, total, "Pay by cash / transfer / card" prompt, auto-refreshed via WebSocket.
4. Everything stays on the LAN. Selling never touches the internet. If A dies, B shows "Server offline" and blocks selling until A is back.

Terminal B never touches the database file — it is purely an HTTP client. If B should later also take orders (second cashier), it's the same plumbing; just open the full POS page instead of `/display`.

## 3. Stack

| Layer | Choice | Why |
|---|---|---|
| Language | Go 1.22+ | Single static .exe, cross-compiles to Windows, easy to maintain |
| HTTP | stdlib `net/http` (+ `chi` if routing middleware is wanted) | No framework needed at this size |
| DB | `modernc.org/sqlite` (pure Go, no CGO) | Cross-compile to Windows without a C toolchain; WAL mode |
| Migrations | Embedded SQL files, tiny runner at startup | Simple and repeatable |
| UI | `templ` or `html/template` + htmx + Tailwind | Server-rendered; no Node build step |
| Live events | Server-Sent Events (stdlib) | One-way push (server → terminals) is all we need; no extra dependency |
| Auth | `golang.org/x/crypto/argon2` PIN hashing + random session tokens | Safe PIN storage, server-side sessions |
| Money/units | `int64` kobo, `int` quantities | No float anywhere |
| Printing | ESC/POS over TCP 9100 (network thermal printer); USB via Windows spooler as fallback | Raw socket write, no driver drama |
| Assets | `embed.FS` | UI ships inside the .exe |
| Logging | `slog` + rolling file | Stdlib, good enough |

## 4. Data model

Every table: UUIDv7 primary key, `created_at`, `updated_at`, `terminal_id`. Money in kobo (integers), quantities in integers.

| Table | Purpose |
|---|---|
| users | Worker name, role (cashier/manager/owner), pin_hash, active |
| categories | Rice, Sides, Pastries, Drinks, Packs; manager can add more |
| items | Name, category, unit_type (spoon/count), price_per_unit_kobo, low_stock_level, active |
| item_prices | Dated price history; past sales keep the price they were sold at |
| packs / pack_components | Saved bundles sold with one tap; stock deducted per component |
| discounts | Percent or fixed; scope (all_food/category/item/pack); schedule or manual; no stacking — best single discount per line |
| settings | VAT on/off, rate (basis points), mode (inclusive/added on top), business details, packaging fee |
| orders / order_lines / order_line_components | Orders with type (dine-in/table or takeaway), totals, worker, terminal; pack components frozen at sale time |
| payments | Cash (change due), bank transfer, card — manual confirmation, optional reference |
| stock_ledger | Append-only: batch_in, sale, void_return, wastage, adjustment, restock, count_diff |
| stock_balances | Cached balance, updated in the same transaction as each ledger row |
| stock_counts | End-of-day counted vs expected, with difference |
| print_jobs | Receipt/duplicate/slip queue with retry |
| audit_log | Who, which terminal, what, when |
| sync_outbox / sync_state | Outbox pattern for cloud push; sync status for the on-screen indicator |

## 5. Project structure

```
tatsy/
├── cmd/posd/main.go        # single entrypoint; flags: -mode server|display, -db, -addr
├── internal/
│   ├── core/               # pure business rules: pricing, VAT, discounts, packs, ledger (most tests here)
│   ├── store/              # SQLite repos, migrations, outbox
│   ├── api/                # handlers, auth middleware, WS hub
│   ├── print/              # ESC/POS builder, printer config, retry queue
│   └── sync/               # outbox worker (Phase 5)
├── web/                    # templ templates + static (kiosk seller UI, /display buyer UI)
└── migrations/
```

## 6. Phases

| Phase | Scope | Done when |
|---|---|---|
| 1. Foundation | SQLite + migrations, PIN login/roles, LAN server on A, seller UI shell, `/display` on B, WS hub, settings | Both PCs connect; B mirrors a test order live |
| 2. Selling | Menu (spoon/count), packs, custom plates, dine-in/takeaway, discounts, VAT both modes, cash/transfer/card, receipts | Full sale to printed receipt on A, mirrored on B |
| 3. Stock & control | Batches, live balances, sold-out greying, wastage, end-of-day count, audit log | Full trading day, every sale tied to a worker |
| 4. Reports & backup | Sales/stock/worker reports, CSV export, scheduled + manual backup/restore | Owner reviews a day and restores from backup |
| 5. Sync (optional/deferrable) | Outbox worker → cloud API, status indicator | Data lands in cloud after outage, no duplicates |
| 6. Pilot | Parallel run vs paper, installer (one .exe + startup shortcut), training | Owner sign-off |

## 7. Open items

1. Confirm the VAT rate and treatment with an accountant (rate is a setting, not hard-coded).
2. Choose printer model — a **network printer** keeps Go printing trivial (raw ESC/POS over TCP 9100).
3. Reserve the fixed IP for Terminal A on the router — prerequisite for Phase 1.
4. Decide what the buyer sees on the `/display` screen (running total, order number, payment prompts, branding).
5. Decide whether Terminal B stays display-only or becomes a second cashier terminal later (same plumbing either way).
