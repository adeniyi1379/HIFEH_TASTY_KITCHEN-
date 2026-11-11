-- 003_inventory.sql
-- Add inventory tracking and stock quantities to menu items
ALTER TABLE items ADD COLUMN track_inventory INTEGER NOT NULL DEFAULT 1;
ALTER TABLE items ADD COLUMN stock_quantity INTEGER NOT NULL DEFAULT 50;

CREATE INDEX IF NOT EXISTS idx_items_stock ON items (track_inventory, stock_quantity);
