CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('cashier','manager','owner')),
    pin_hash    TEXT NOT NULL,
    active      INTEGER NOT NULL DEFAULT 1,
    terminal_id TEXT NOT NULL DEFAULT 'T1',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS categories (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS items (
    id                TEXT PRIMARY KEY,
    category_id       TEXT NOT NULL REFERENCES categories(id),
    name              TEXT NOT NULL,
    unit_type         TEXT NOT NULL CHECK (unit_type IN ('spoon','count')),
    unit_label        TEXT NOT NULL DEFAULT 'spoon',
    price_per_unit_kobo INTEGER NOT NULL,
    active            INTEGER NOT NULL DEFAULT 1,
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS settings (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    business_name       TEXT NOT NULL DEFAULT 'Tatsy Kitchen',
    receipt_footer      TEXT NOT NULL DEFAULT 'Thank you for dining with us!',
    vat_enabled         INTEGER NOT NULL DEFAULT 0,
    vat_rate_bp         INTEGER NOT NULL DEFAULT 750,
    vat_mode            TEXT NOT NULL DEFAULT 'inclusive' CHECK (vat_mode IN ('inclusive','added_on_top')),
    packaging_fee_kobo  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sequences (
    name  TEXT PRIMARY KEY,
    value INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
    id           TEXT PRIMARY KEY,
    order_no     TEXT NOT NULL UNIQUE,
    type         TEXT NOT NULL DEFAULT 'dine_in' CHECK (type IN ('dine_in','takeaway')),
    table_no     TEXT,
    status       TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','displayed','paid','void')),
    subtotal_kobo INTEGER NOT NULL DEFAULT 0,
    vat_kobo     INTEGER NOT NULL DEFAULT 0,
    total_kobo   INTEGER NOT NULL DEFAULT 0,
    user_id      TEXT REFERENCES users(id),
    terminal_id  TEXT NOT NULL DEFAULT 'T1',
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS order_lines (
    id             TEXT PRIMARY KEY,
    order_id       TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    item_id        TEXT NOT NULL REFERENCES items(id),
    name_snapshot  TEXT NOT NULL,
    unit_label     TEXT NOT NULL,
    quantity       INTEGER NOT NULL,
    unit_price_kobo INTEGER NOT NULL,
    line_total_kobo INTEGER NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (datetime('now'))
);
