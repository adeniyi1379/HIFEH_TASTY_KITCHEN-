package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
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
	ID               string           `json:"id"`
	CategoryID       string           `json:"category_id"`
	CategoryName     string           `json:"category_name,omitempty"`
	Name             string           `json:"name"`
	UnitType         string           `json:"unit_type"`
	UnitLabel        string           `json:"unit_label"`
	PricePerUnitKobo int64            `json:"price_per_unit_kobo"`
	Active           bool             `json:"active"`
	TrackInventory   bool             `json:"track_inventory"`
	StockQuantity    int64            `json:"stock_quantity"`
	IsCombo          bool             `json:"is_combo"`
	Components       []ComboComponent `json:"components,omitempty"`
	ComponentsJSON   string           `json:"-"`
}

// ComboComponent is one menu item bundled inside a combo, with how many
// of it each combo serves.
type ComboComponent struct {
	ItemID   string `json:"item_id"`
	Name     string `json:"name"`
	Quantity int64  `json:"quantity"`
}

// loadComboComponents returns the components of a combo, sorted.
func loadComboComponents(ctx context.Context, q queryer, comboID string) ([]ComboComponent, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT cc.item_id, COALESCE(i.name, ''), cc.quantity
		 FROM combo_components cc LEFT JOIN items i ON i.id = cc.item_id
		 WHERE cc.combo_id = ? ORDER BY cc.sort_order, i.name`, comboID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ComboComponent
	for rows.Next() {
		var c ComboComponent
		if err := rows.Scan(&c.ItemID, &c.Name, &c.Quantity); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// attachComponents loads components and their JSON form for every combo
// item in the slice (one query for all).
func attachComponents(ctx context.Context, db *sql.DB, items []Item) {
	ids := make([]string, 0, len(items))
	for _, it := range items {
		if it.IsCombo {
			ids = append(ids, it.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	ph := strings.Repeat("?,", len(ids))
	ph = ph[:len(ph)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.QueryContext(ctx,
		`SELECT cc.combo_id, cc.item_id, COALESCE(i.name, ''), cc.quantity
		 FROM combo_components cc LEFT JOIN items i ON i.id = cc.item_id
		 WHERE cc.combo_id IN (`+ph+`) ORDER BY cc.sort_order, i.name`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	byCombo := make(map[string][]ComboComponent)
	for rows.Next() {
		var comboID string
		var c ComboComponent
		if rows.Scan(&comboID, &c.ItemID, &c.Name, &c.Quantity) == nil {
			byCombo[comboID] = append(byCombo[comboID], c)
		}
	}
	for i := range items {
		if !items[i].IsCombo {
			continue
		}
		items[i].Components = byCombo[items[i].ID]
		if items[i].Components == nil {
			items[i].Components = []ComboComponent{}
		}
		if b, err := json.Marshal(items[i].Components); err == nil {
			items[i].ComponentsJSON = string(b)
		}
	}
}

func replaceComboComponents(ctx context.Context, tx *sql.Tx, comboID string, comps []ComboComponent) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM combo_components WHERE combo_id = ?`, comboID); err != nil {
		return err
	}
	for i, c := range comps {
		if c.Quantity <= 0 {
			c.Quantity = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO combo_components (combo_id, item_id, quantity, sort_order) VALUES (?,?,?,?)`,
			comboID, c.ItemID, c.Quantity, i); err != nil {
			return err
		}
	}
	return nil
}

// validateComboComponents checks that every component exists, is active,
// is not a combo itself, and that the combo does not reference itself.
func validateComboComponents(ctx context.Context, q queryer, comboID string, comps []ComboComponent) error {
	seen := make(map[string]bool)
	for _, c := range comps {
		if c.ItemID == "" {
			return errors.New("combo component item is required")
		}
		if c.ItemID == comboID {
			return errors.New("a combo cannot include itself")
		}
		if seen[c.ItemID] {
			return errors.New("duplicate combo component")
		}
		seen[c.ItemID] = true
		var isCombo int
		err := q.QueryRowContext(ctx,
			`SELECT is_combo FROM items WHERE id = ? AND active = 1`, c.ItemID).Scan(&isCombo)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("combo component not found or inactive: %s", c.ItemID)
		}
		if err != nil {
			return err
		}
		if isCombo == 1 {
			return errors.New("combo components cannot themselves be combos")
		}
	}
	return nil
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

const itemColumns = `i.id, i.category_id, COALESCE(c.name, ''), i.name, i.unit_type, i.unit_label, i.price_per_unit_kobo, i.active, i.track_inventory, i.stock_quantity, i.is_combo`

func scanItem(sc func(...any) error) (Item, error) {
	var it Item
	var act, track, combo int
	if err := sc(&it.ID, &it.CategoryID, &it.CategoryName, &it.Name, &it.UnitType, &it.UnitLabel, &it.PricePerUnitKobo, &act, &track, &it.StockQuantity, &combo); err != nil {
		return Item{}, err
	}
	it.Active = act == 1
	it.TrackInventory = track == 1
	it.IsCombo = combo == 1
	return it, nil
}

func ListItems(ctx context.Context, db *sql.DB) ([]Item, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+itemColumns+`
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
		it, err := scanItem(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	attachComponents(ctx, db, out)
	return out, nil
}

func ListAllItems(ctx context.Context, db *sql.DB) ([]Item, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+itemColumns+`
		 FROM items i
		 LEFT JOIN categories c ON c.id = i.category_id
		 ORDER BY c.sort_order, i.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		it, err := scanItem(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	attachComponents(ctx, db, out)
	return out, nil
}

type ComboComponentInput struct {
	ItemID   string `json:"item_id"`
	Quantity int64  `json:"quantity"`
}

func CreateItem(ctx context.Context, db *sql.DB, categoryID, name, unitType, unitLabel string, priceKobo int64, trackInventory bool, stockQuantity int64, isCombo bool, components []ComboComponentInput) (Item, error) {
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

	// Combos are assembled from other items; stock lives on the
	// components, never on the bundle itself.
	if isCombo {
		trackInventory = false
		stockQuantity = 0
		if len(components) == 0 {
			return Item{}, errors.New("a combo needs at least one component")
		}
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
	comboInt := 0
	if isCombo {
		comboInt = 1
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback()

	if err := validateComboComponents(ctx, tx, id, toComboComponents(components)); err != nil {
		return Item{}, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO items (id, category_id, name, unit_type, unit_label, price_per_unit_kobo, active, track_inventory, stock_quantity, is_combo)
		 VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?)`,
		id, categoryID, name, unitType, unitLabel, priceKobo, trackInt, stockQuantity, comboInt,
	); err != nil {
		return Item{}, err
	}
	if isCombo {
		if err := replaceComboComponents(ctx, tx, id, toComboComponents(components)); err != nil {
			return Item{}, err
		}
	}
	if err := tx.Commit(); err != nil {
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
		IsCombo:          isCombo,
		Components:       []ComboComponent{},
	}, nil
}

func toComboComponents(in []ComboComponentInput) []ComboComponent {
	out := make([]ComboComponent, len(in))
	for i, c := range in {
		out[i] = ComboComponent{ItemID: c.ItemID, Quantity: c.Quantity}
	}
	return out
}

func UpdateItem(ctx context.Context, db *sql.DB, id, categoryID, name, unitType, unitLabel string, priceKobo int64, trackInventory bool, stockQuantity int64, isCombo bool, components []ComboComponentInput) (Item, error) {
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

	// Combos are assembled from other items; stock lives on the
	// components, never on the bundle itself.
	if isCombo {
		trackInventory = false
		stockQuantity = 0
		if len(components) == 0 {
			return Item{}, errors.New("a combo needs at least one component")
		}
	}

	trackInt := 0
	if trackInventory {
		trackInt = 1
	}
	comboInt := 0
	if isCombo {
		comboInt = 1
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback()

	if err := validateComboComponents(ctx, tx, id, toComboComponents(components)); err != nil {
		return Item{}, err
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE items SET category_id = ?, name = ?, unit_type = ?, unit_label = ?, price_per_unit_kobo = ?, track_inventory = ?, stock_quantity = ?, is_combo = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		categoryID, name, unitType, unitLabel, priceKobo, trackInt, stockQuantity, comboInt, id,
	)
	if err != nil {
		return Item{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Item{}, errors.New("item not found")
	}
	if isCombo {
		if err := replaceComboComponents(ctx, tx, id, toComboComponents(components)); err != nil {
			return Item{}, err
		}
	} else if _, err := tx.ExecContext(ctx, `DELETE FROM combo_components WHERE combo_id = ?`, id); err != nil {
		return Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return Item{}, err
	}

	var it Item
	var act, track, combo int
	err = db.QueryRowContext(ctx,
		`SELECT `+itemColumns+`
		 FROM items i LEFT JOIN categories c ON c.id = i.category_id WHERE i.id = ?`, id,
	).Scan(&it.ID, &it.CategoryID, &it.CategoryName, &it.Name, &it.UnitType, &it.UnitLabel, &it.PricePerUnitKobo, &act, &track, &it.StockQuantity, &combo)
	if err != nil {
		return Item{}, err
	}
	it.Active = act == 1
	it.TrackInventory = track == 1
	it.IsCombo = combo == 1
	if it.IsCombo {
		comps, err := loadComboComponents(ctx, db, it.ID)
		if err != nil {
			return Item{}, err
		}
		it.Components = comps
	}
	return it, nil
}

// GetItemStock returns the current stock quantity of an item.
func GetItemStock(ctx context.Context, db *sql.DB, id string) (int64, error) {
	var stock int64
	err := db.QueryRowContext(ctx, `SELECT stock_quantity FROM items WHERE id = ?`, id).Scan(&stock)
	return stock, err
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
	var usedIn int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM combo_components cc JOIN items i ON i.id = cc.combo_id
		 WHERE cc.item_id = ? AND i.is_combo = 1`, id).Scan(&usedIn)
	if err != nil {
		return err
	}
	if usedIn > 0 {
		return errors.New("cannot delete item used in a combo; remove it from the combo first")
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
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure: fall back to a timestamp suffix so id
		// uniqueness does not depend on randomness alone.
		return fmt.Sprintf("%x", time.Now().UnixNano())[:n]
	}
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
