-- 004_order_edit.sql
ALTER TABLE orders ADD COLUMN is_edit INTEGER NOT NULL DEFAULT 0;
