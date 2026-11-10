package store

import (
	"context"
	"database/sql"
)

// Seed inserts the starting menu, accounts and settings on first run only.
// Everything is idempotent: running it twice changes nothing.
func Seed(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if err := seedUser(tx, "Owner", "owner", "123456"); err != nil {
			return err
		}
		if err := seedUser(tx, "Manager Ada", "manager", "234567"); err != nil {
			return err
		}
		if err := seedUser(tx, "Amaka", "cashier", "345678"); err != nil {
			return err
		}
		if err := seedUser(tx, "Tunde", "cashier", "456789"); err != nil {
			return err
		}
	}

	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		cats := []struct{ id, name string; sort int }{
			{"cat_rice", "Rice", 1},
			{"cat_sides", "Sides", 2},
			{"cat_pastries", "Pastries", 3},
			{"cat_drinks", "Drinks", 4},
		}
		for _, c := range cats {
			if _, err := tx.ExecContext(ctx, `INSERT INTO categories (id, name, sort_order) VALUES (?,?,?)`, c.id, c.name, c.sort); err != nil {
				return err
			}
		}
		items := []struct {
			id, cat, name, unit, label string
			price                      int64
		}{
			{"itm_jollof", "cat_rice", "Jollof Rice", "spoon", "spoon", 50000},
			{"itm_fried", "cat_rice", "Fried Rice", "spoon", "spoon", 50000},
			{"itm_dodo", "cat_sides", "Dodo", "count", "piece", 10000},
			{"itm_salad", "cat_sides", "Salad", "spoon", "spoon", 25000},
			{"itm_meatpie", "cat_pastries", "Meat Pie", "count", "piece", 35000},
			{"itm_eggroll", "cat_pastries", "Egg Roll", "count", "piece", 30000},
			{"itm_coke", "cat_drinks", "Coke", "count", "bottle", 30000},
			{"itm_fanta", "cat_drinks", "Fanta", "count", "bottle", 30000},
			{"itm_water", "cat_drinks", "Water", "count", "bottle", 20000},
		}
		for _, it := range items {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO items (id, category_id, name, unit_type, unit_label, price_per_unit_kobo) VALUES (?,?,?,?,?,?)`,
				it.id, it.cat, it.name, it.unit, it.label, it.price); err != nil {
				return err
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO settings (id) VALUES (1)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO sequences (name, value) VALUES ('T1', 0)`); err != nil {
		return err
	}
	return tx.Commit()
}

func seedUser(tx *sql.Tx, name, role, pin string) error {
	hash, err := HashPIN(pin)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO users (id, name, role, pin_hash) VALUES (?,?,?,?)`,
		NewID(), name, role, hash)
	return err
}
