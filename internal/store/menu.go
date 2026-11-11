package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Settings struct {
	BusinessName     string `json:"business_name"`
	ReceiptFooter    string `json:"receipt_footer"`
	VATEnabled       bool   `json:"vat_enabled"`
	VATRateBP        int64  `json:"vat_rate_bp"`
	VATMode          string `json:"vat_mode"`
	PackagingFeeKobo int64  `json:"packaging_fee_kobo"`
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

func UpdateSettings(ctx context.Context, db *sql.DB, businessName, receiptFooter string, vatEnabled bool, vatRateBP int64, vatMode string, packagingFeeKobo int64) (Settings, error) {
	if businessName == "" {
		return Settings{}, errors.New("business name cannot be empty")
	}
	if vatMode != "inclusive" && vatMode != "added_on_top" {
		vatMode = "inclusive"
	}
	vatInt := 0
	if vatEnabled {
		vatInt = 1
	}
	_, err := db.ExecContext(ctx,
		`UPDATE settings SET business_name = ?, receipt_footer = ?, vat_enabled = ?, vat_rate_bp = ?, vat_mode = ?, packaging_fee_kobo = ? WHERE id = 1`,
		businessName, receiptFooter, vatInt, vatRateBP, vatMode, packagingFeeKobo,
	)
	if err != nil {
		return Settings{}, err
	}
	return GetSettings(ctx, db)
}

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type Item struct {
	ID               string `json:"id"`
	CategoryID       string `json:"category_id"`
	CategoryName     string `json:"category_name,omitempty"`
	Name             string `json:"name"`
	UnitType         string `json:"unit_type"`
	UnitLabel        string `json:"unit_label"`
	PricePerUnitKobo int64  `json:"price_per_unit_kobo"`
	Active           bool   `json:"active"`
	TrackInventory   bool   `json:"track_inventory"`
	StockQuantity    int64  `json:"stock_quantity"`
}

func ListCategories(ctx context.Context, db *sql.DB) ([]Category, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, sort_order FROM categories ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func CreateCategory(ctx context.Context, db *sql.DB, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, errors.New("category name cannot be empty")
	}
	slug := slugify(name)
	id := "cat_" + slug
	if len(id) > 24 {
		id = id[:24]
	}

	var maxSort int
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort_order), 0) FROM categories`).Scan(&maxSort)
	sortOrder := maxSort + 1

	// Ensure unique id
	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id = ?`, id).Scan(&count)
	if count > 0 {
		id = fmt.Sprintf("cat_%s_%s", slug, randHex(4))
	}

	_, err := db.ExecContext(ctx, `INSERT INTO categories (id, name, sort_order) VALUES (?, ?, ?)`, id, name, sortOrder)
	if err != nil {
		return Category{}, err
	}
	return Category{ID: id, Name: name, SortOrder: sortOrder}, nil
}

func UpdateCategory(ctx context.Context, db *sql.DB, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("category name cannot be empty")
	}
	res, err := db.ExecContext(ctx, `UPDATE categories SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("category not found")
	}
	return nil
}

func DeleteCategory(ctx context.Context, db *sql.DB, id string) error {
	var itemCount int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE category_id = ?`, id).Scan(&itemCount)
	if err != nil {
		return err
	}
	if itemCount > 0 {
		return errors.New("cannot delete category with existing items")
	}
	res, err := db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("category not found")
	}
	return nil
}

func ListItems(ctx context.Context, db *sql.DB) ([]Item, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT i.id, i.category_id, COALESCE(c.name, ''), i.name, i.unit_type, i.unit_label, i.price_per_unit_kobo, i.active, i.track_inventory, i.stock_quantity
		 FROM items i
		 LEFT JOIN categories c ON c.id = i.category_id
		 WHERE i.active = 1
		 ORDER BY i.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		var act, track int
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.CategoryName, &it.Name, &it.UnitType, &it.UnitLabel, &it.PricePerUnitKobo, &act, &track, &it.StockQuantity); err != nil {
			return nil, err
		}
		it.Active = act == 1
		it.TrackInventory = track == 1
		out = append(out, it)
	}
	return out, rows.Err()
}

func ListAllItems(ctx context.Context, db *sql.DB) ([]Item, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT i.id, i.category_id, COALESCE(c.name, ''), i.name, i.unit_type, i.unit_label, i.price_per_unit_kobo, i.active, i.track_inventory, i.stock_quantity
		 FROM items i
		 LEFT JOIN categories c ON c.id = i.category_id
		 ORDER BY c.sort_order, i.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		var act, track int
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.CategoryName, &it.Name, &it.UnitType, &it.UnitLabel, &it.PricePerUnitKobo, &act, &track, &it.StockQuantity); err != nil {
			return nil, err
		}
		it.Active = act == 1
		it.TrackInventory = track == 1
		out = append(out, it)
	}
	return out, rows.Err()
}

func CreateItem(ctx context.Context, db *sql.DB, categoryID, name, unitType, unitLabel string, priceKobo int64, trackInventory bool, stockQuantity int64) (Item, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Item{}, errors.New("item name cannot be empty")
	}
	if categoryID == "" {
		return Item{}, errors.New("category is required")
	}
	if unitType != "spoon" && unitType != "count" {
		unitType = "count"
	}
	if unitLabel == "" {
		if unitType == "spoon" {
			unitLabel = "spoon"
		} else {
			unitLabel = "piece"
		}
	}
	if priceKobo < 0 {
		return Item{}, errors.New("price cannot be negative")
	}
	if stockQuantity < 0 {
		stockQuantity = 0
	}

	slug := slugify(name)
	id := "itm_" + slug
	if len(id) > 20 {
		id = id[:20]
	}

	var count int
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE id = ?`, id).Scan(&count)
	if count > 0 {
		id = fmt.Sprintf("itm_%s_%s", slug, randHex(4))
	}

	trackInt := 0
	if trackInventory {
		trackInt = 1
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO items (id, category_id, name, unit_type, unit_label, price_per_unit_kobo, active, track_inventory, stock_quantity)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		id, categoryID, name, unitType, unitLabel, priceKobo, trackInt, stockQuantity,
	)
	if err != nil {
		return Item{}, err
	}
	return Item{
		ID:               id,
		CategoryID:       categoryID,
		Name:             name,
		UnitType:         unitType,
		UnitLabel:        unitLabel,
		PricePerUnitKobo: priceKobo,
		Active:           true,
		TrackInventory:   trackInventory,
		StockQuantity:    stockQuantity,
	}, nil
}

func UpdateItem(ctx context.Context, db *sql.DB, id, categoryID, name, unitType, unitLabel string, priceKobo int64, trackInventory bool, stockQuantity int64) (Item, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Item{}, errors.New("item name cannot be empty")
	}
	if unitType != "spoon" && unitType != "count" {
		unitType = "count"
	}
	if unitLabel == "" {
		unitLabel = "piece"
	}
	if priceKobo < 0 {
		return Item{}, errors.New("price cannot be negative")
	}
	if stockQuantity < 0 {
		stockQuantity = 0
	}

	trackInt := 0
	if trackInventory {
		trackInt = 1
	}

	res, err := db.ExecContext(ctx,
		`UPDATE items SET category_id = ?, name = ?, unit_type = ?, unit_label = ?, price_per_unit_kobo = ?, track_inventory = ?, stock_quantity = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		categoryID, name, unitType, unitLabel, priceKobo, trackInt, stockQuantity, id,
	)
	if err != nil {
		return Item{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Item{}, errors.New("item not found")
	}

	var it Item
	var act, track int
	err = db.QueryRowContext(ctx,
		`SELECT i.id, i.category_id, COALESCE(c.name, ''), i.name, i.unit_type, i.unit_label, i.price_per_unit_kobo, i.active, i.track_inventory, i.stock_quantity
		 FROM items i LEFT JOIN categories c ON c.id = i.category_id WHERE i.id = ?`, id,
	).Scan(&it.ID, &it.CategoryID, &it.CategoryName, &it.Name, &it.UnitType, &it.UnitLabel, &it.PricePerUnitKobo, &act, &track, &it.StockQuantity)
	it.Active = act == 1
	it.TrackInventory = track == 1
	return it, err
}

func UpdateItemStock(ctx context.Context, db *sql.DB, id string, stockQuantity int64) (int64, error) {
	if stockQuantity < 0 {
		stockQuantity = 0
	}
	res, err := db.ExecContext(ctx, `UPDATE items SET stock_quantity = ?, updated_at = datetime('now') WHERE id = ?`, stockQuantity, id)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return 0, errors.New("item not found")
	}
	return stockQuantity, nil
}

func RestockItem(ctx context.Context, db *sql.DB, id string, delta int64) (int64, int64, error) {
	var currentStock int64
	err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM items WHERE id = ?`, id).Scan(&currentStock)
	if err != nil {
		return 0, 0, err
	}
	newStock := currentStock + delta
	if newStock < 0 {
		newStock = 0
	}
	_, err = db.ExecContext(ctx, `UPDATE items SET stock_quantity = ?, updated_at = datetime('now') WHERE id = ?`, newStock, id)
	return currentStock, newStock, err
}

func ToggleItemInventoryTracking(ctx context.Context, db *sql.DB, id string, track bool) error {
	t := 0
	if track {
		t = 1
	}
	res, err := db.ExecContext(ctx, `UPDATE items SET track_inventory = ?, updated_at = datetime('now') WHERE id = ?`, t, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("item not found")
	}
	return nil
}

func SetItemActive(ctx context.Context, db *sql.DB, id string, active bool) error {
	act := 0
	if active {
		act = 1
	}
	res, err := db.ExecContext(ctx, `UPDATE items SET active = ?, updated_at = datetime('now') WHERE id = ?`, act, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("item not found")
	}
	return nil
}

func DeleteItem(ctx context.Context, db *sql.DB, id string) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM order_lines WHERE item_id = ?`, id).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("cannot delete item with existing sales history; deactivate it instead")
	}
	res, err := db.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("item not found")
	}
	return nil
}

func randHex(n int) string {
	b := make([]byte, (n+1)/2)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonWord.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "item"
	}
	return s
}
