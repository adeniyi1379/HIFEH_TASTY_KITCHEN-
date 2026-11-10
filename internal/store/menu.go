package store

import (
	"context"
	"database/sql"
)

type Settings struct {
	BusinessName      string
	ReceiptFooter     string
	VATEnabled        bool
	VATRateBP         int64
	VATMode           string
	PackagingFeeKobo  int64
}

func GetSettings(ctx context.Context, db *sql.DB) (Settings, error) {
	var s Settings
	var vatEnabled int
	err := db.QueryRowContext(ctx,
		`SELECT business_name, receipt_footer, vat_enabled, vat_rate_bp, vat_mode, packaging_fee_kobo FROM settings WHERE id = 1`,
	).Scan(&s.BusinessName, &s.ReceiptFooter, &vatEnabled, &s.VATRateBP, &s.VATMode, &s.PackagingFeeKobo)
	s.VATEnabled = vatEnabled == 1
	return s, err
}

type Category struct {
	ID   string
	Name string
}

type Item struct {
	ID              string
	CategoryID      string
	Name            string
	UnitType        string
	UnitLabel       string
	PricePerUnitKobo int64
}

func ListCategories(ctx context.Context, db *sql.DB) ([]Category, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name FROM categories ORDER BY sort_order`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func ListItems(ctx context.Context, db *sql.DB) ([]Item, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, category_id, name, unit_type, unit_label, price_per_unit_kobo FROM items WHERE active = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.Name, &it.UnitType, &it.UnitLabel, &it.PricePerUnitKobo); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
