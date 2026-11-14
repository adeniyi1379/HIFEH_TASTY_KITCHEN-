-- 005_combo.sql
-- Meal combos: an item flagged is_combo bundles other menu items.
-- Choosing a combo on the till automatically adds its components.
ALTER TABLE items ADD COLUMN is_combo INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS combo_components (
    combo_id   TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    item_id    TEXT NOT NULL REFERENCES items(id),
    quantity   INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (combo_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_combo_components_item ON combo_components (item_id);
