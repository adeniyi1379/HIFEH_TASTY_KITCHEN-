package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tatsy/internal/core"
)

var ErrNoOpenOrder = errors.New("no open order")

// queryer is satisfied by both *sql.DB and *sql.Tx.
type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type OrderLine struct {
	ID            string `json:"id"`
	ItemID        string `json:"item_id"`
	Name          string `json:"name"`
	UnitLabel     string `json:"unit_label"`
	Quantity      int64  `json:"quantity"`
	UnitPriceKobo int64  `json:"unit_price_kobo"`
	LineTotalKobo int64  `json:"line_total_kobo"`
}

type Order struct {
	ID           string      `json:"id"`
	OrderNo      string      `json:"order_no"`
	Type         string      `json:"type"`
	TableNo      *string     `json:"table_no"`
	Status       string      `json:"status"`
	UserID       string      `json:"user_id"`
	UserName     string      `json:"user_name"`
	IsEdit       bool        `json:"is_edit"`
	SubtotalKobo int64       `json:"subtotal_kobo"`
	VATKobo      int64       `json:"vat_kobo"`
	TotalKobo    int64       `json:"total_kobo"`
	Lines        []OrderLine `json:"lines"`
}

// GetLatestOrder returns the most recent order that is not void
// (what the buyer display mirrors), or ErrNoOpenOrder.
func GetLatestOrder(ctx context.Context, db *sql.DB) (*Order, error) {
	var id string
	// Prioritize currently open or displayed orders, sorted by most recently updated
	err := db.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE status IN ('open','displayed') ORDER BY updated_at DESC, created_at DESC LIMIT 1`,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		err = db.QueryRowContext(ctx,
			`SELECT id FROM orders WHERE status IN ('open','displayed','paid') ORDER BY updated_at DESC, created_at DESC LIMIT 1`,
		).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoOpenOrder
	}
	if err != nil {
		return nil, err
	}
	return getOrder(ctx, db, id)
}

func GetOrderByID(ctx context.Context, db *sql.DB, id string) (*Order, error) {
	return getOrder(ctx, db, id)
}

func ReopenOrder(ctx context.Context, db *sql.DB, orderID string) (*Order, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var prevStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = ?`, orderID).Scan(&prevStatus)
	if err != nil {
		return nil, err
	}

	// If order was paid, restore the deducted inventory
	if prevStatus == "paid" {
		rows, err := tx.QueryContext(ctx, `SELECT item_id, quantity FROM order_lines WHERE order_id = ?`, orderID)
		if err != nil {
			return nil, err
		}
		type lineQty struct {
			itemID string
			qty    int64
		}
		var lines []lineQty
		for rows.Next() {
			var l lineQty
			if err := rows.Scan(&l.itemID, &l.qty); err == nil {
				lines = append(lines, l)
			}
		}
		rows.Close()

		for _, l := range lines {
			if _, err := tx.ExecContext(ctx,
				`UPDATE items SET stock_quantity = stock_quantity + ?, updated_at = datetime('now') WHERE id = ? AND track_inventory = 1`,
				l.qty, l.itemID); err != nil {
				return nil, err
			}
		}
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE orders SET status = 'open', is_edit = 1, updated_at = datetime('now') WHERE id = ?`, orderID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, errors.New("order not found")
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

func VoidOrder(ctx context.Context, db *sql.DB, orderID string) (*Order, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var prevStatus string
	err = tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = ?`, orderID).Scan(&prevStatus)
	if err != nil {
		return nil, err
	}
	if prevStatus == "void" {
		return nil, errors.New("order is already void")
	}

	// If order was paid, restore the deducted inventory
	if prevStatus == "paid" {
		rows, err := tx.QueryContext(ctx, `SELECT item_id, quantity FROM order_lines WHERE order_id = ?`, orderID)
		if err != nil {
			return nil, err
		}
		type lineQty struct {
			itemID string
			qty    int64
		}
		var lines []lineQty
		for rows.Next() {
			var l lineQty
			if err := rows.Scan(&l.itemID, &l.qty); err == nil {
				lines = append(lines, l)
			}
		}
		rows.Close()

		for _, l := range lines {
			if _, err := tx.ExecContext(ctx,
				`UPDATE items SET stock_quantity = stock_quantity + ?, updated_at = datetime('now') WHERE id = ? AND track_inventory = 1`,
				l.qty, l.itemID); err != nil {
				return nil, err
			}
		}
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE orders SET status = 'void', updated_at = datetime('now') WHERE id = ?`, orderID)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, errors.New("order not found")
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

func ListRecentOrders(ctx context.Context, db *sql.DB, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id FROM orders ORDER BY updated_at DESC, created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()

	var orders []Order
	for _, id := range ids {
		o, err := getOrder(ctx, db, id)
		if err == nil && o != nil {
			orders = append(orders, *o)
		}
	}
	return orders, nil
}

func getOrder(ctx context.Context, q queryer, id string) (*Order, error) {
	o := &Order{}
	var tableNo, userID, userName sql.NullString
	var isEdit int
	err := q.QueryRowContext(ctx,
		`SELECT o.id, o.order_no, o.type, o.table_no, o.status, o.user_id, u.name, o.is_edit, o.subtotal_kobo, o.vat_kobo, o.total_kobo
		 FROM orders o LEFT JOIN users u ON u.id = o.user_id WHERE o.id = ?`, id,
	).Scan(&o.ID, &o.OrderNo, &o.Type, &tableNo, &o.Status, &userID, &userName, &isEdit, &o.SubtotalKobo, &o.VATKobo, &o.TotalKobo)
	if err != nil {
		return nil, err
	}
	if tableNo.Valid {
		o.TableNo = &tableNo.String
	}
	o.UserID = userID.String
	o.UserName = userName.String
	o.IsEdit = isEdit == 1
	rows, err := q.QueryContext(ctx,
		`SELECT id, item_id, name_snapshot, unit_label, quantity, unit_price_kobo, line_total_kobo
		 FROM order_lines WHERE order_id = ? ORDER BY created_at`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	o.Lines = []OrderLine{}
	for rows.Next() {
		var l OrderLine
		if err := rows.Scan(&l.ID, &l.ItemID, &l.Name, &l.UnitLabel, &l.Quantity, &l.UnitPriceKobo, &l.LineTotalKobo); err != nil {
			return nil, err
		}
		o.Lines = append(o.Lines, l)
	}
	return o, rows.Err()
}

// AddItem adds one unit of an item to the current order, creating the
// order (with its sequence number) when this is the first line.
func AddItem(ctx context.Context, db *sql.DB, itemID, userID string) (*Order, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	orderID, err := ensureOpenOrder(ctx, tx, userID)
	if err != nil {
		return nil, err
	}

	var item Item
	var trackInv int
	var stockQty int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, category_id, name, unit_type, unit_label, price_per_unit_kobo, track_inventory, stock_quantity 
		 FROM items WHERE id = ? AND active = 1`, itemID,
	).Scan(&item.ID, &item.CategoryID, &item.Name, &item.UnitType, &item.UnitLabel, &item.PricePerUnitKobo, &trackInv, &stockQty)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("unknown item %q", itemID)
	}
	if err != nil {
		return nil, err
	}
	item.TrackInventory = trackInv == 1
	item.StockQuantity = stockQty

	if item.TrackInventory && item.StockQuantity <= 0 {
		return nil, fmt.Errorf("Out of stock: %s is finished", item.Name)
	}

	var lineID string
	var existingQty int64
	err = tx.QueryRowContext(ctx,
		`SELECT id, quantity FROM order_lines WHERE order_id = ? AND item_id = ?`, orderID, itemID).Scan(&lineID, &existingQty)

	if item.TrackInventory && (existingQty+1) > item.StockQuantity {
		return nil, fmt.Errorf("Cannot add more: only %d %s(s) remaining in stock for %s", item.StockQuantity, item.UnitLabel, item.Name)
	}

	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx,
			`INSERT INTO order_lines (id, order_id, item_id, name_snapshot, unit_label, quantity, unit_price_kobo, line_total_kobo)
			 VALUES (?,?,?,?,?,1,?,?)`,
			NewID(), orderID, item.ID, item.Name, item.UnitLabel, item.PricePerUnitKobo, item.PricePerUnitKobo)
	case err != nil:
	default:
		_, err = tx.ExecContext(ctx,
			`UPDATE order_lines SET quantity = quantity + 1, line_total_kobo = (quantity + 1) * unit_price_kobo WHERE id = ?`, lineID)
	}
	if err != nil {
		return nil, err
	}

	if err := recalcTotals(ctx, tx, orderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

// ChangeLineQty adjusts a line by delta; the line is removed at zero.
func ChangeLineQty(ctx context.Context, db *sql.DB, lineID string, delta int64) (*Order, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var orderID, itemID string
	var qty int64
	err = tx.QueryRowContext(ctx, `SELECT order_id, item_id, quantity FROM order_lines WHERE id = ?`, lineID).Scan(&orderID, &itemID, &qty)
	if err != nil {
		return nil, err
	}
	next := qty + delta

	if delta > 0 {
		var name, unitLabel string
		var trackInv int
		var stockQty int64
		err = tx.QueryRowContext(ctx,
			`SELECT name, unit_label, track_inventory, stock_quantity FROM items WHERE id = ?`, itemID,
		).Scan(&name, &unitLabel, &trackInv, &stockQty)
		if err == nil && trackInv == 1 {
			if next > stockQty {
				return nil, fmt.Errorf("Cannot add more: only %d %s(s) remaining in stock for %s", stockQty, unitLabel, name)
			}
		}
	}

	if next <= 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM order_lines WHERE id = ?`, lineID); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx,
			`UPDATE order_lines SET quantity = ?, line_total_kobo = quantity * unit_price_kobo WHERE id = ?`, next, lineID); err != nil {
			return nil, err
		}
	}
	if err := recalcTotals(ctx, tx, orderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

func RemoveLine(ctx context.Context, db *sql.DB, lineID string) (*Order, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var orderID string
	if err := tx.QueryRowContext(ctx, `SELECT order_id FROM order_lines WHERE id = ?`, lineID).Scan(&orderID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM order_lines WHERE id = ?`, lineID); err != nil {
		return nil, err
	}
	if err := recalcTotals(ctx, tx, orderID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

func SetOrderMeta(ctx context.Context, db *sql.DB, orderID, orderType, tableNo string) (*Order, error) {
	var t any
	if tableNo != "" {
		t = tableNo
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE orders SET type = ?, table_no = ?, updated_at = datetime('now') WHERE id = ?`,
		orderType, t, orderID); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

// SendToDisplay marks the order visible on the buyer terminal.
func SendToDisplay(ctx context.Context, db *sql.DB, orderID string) (*Order, error) {
	if _, err := db.ExecContext(ctx,
		`UPDATE orders SET status = 'displayed', updated_at = datetime('now') WHERE id = ?`, orderID); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

func PayOrder(ctx context.Context, db *sql.DB, orderID string) (*Order, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM orders WHERE id = ?`, orderID).Scan(&status)
	if err != nil {
		return nil, err
	}
	if status == "paid" {
		return nil, errors.New("order is already paid")
	}

	// Fetch lines
	rows, err := tx.QueryContext(ctx, `SELECT item_id, quantity FROM order_lines WHERE order_id = ?`, orderID)
	if err != nil {
		return nil, err
	}
	type itemQty struct {
		itemID string
		qty    int64
	}
	var lines []itemQty
	for rows.Next() {
		var l itemQty
		if err := rows.Scan(&l.itemID, &l.qty); err != nil {
			rows.Close()
			return nil, err
		}
		lines = append(lines, l)
	}
	rows.Close()

	// Check stock
	for _, l := range lines {
		var name string
		var trackInv int
		var stockQty int64
		err := tx.QueryRowContext(ctx,
			`SELECT name, track_inventory, stock_quantity FROM items WHERE id = ?`, l.itemID,
		).Scan(&name, &trackInv, &stockQty)
		if err == nil && trackInv == 1 {
			if stockQty < l.qty {
				return nil, fmt.Errorf("Cannot complete payment: %s is out of stock (only %d left)", name, stockQty)
			}
		}
	}

	// Deduct stock for tracked items
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx,
			`UPDATE items SET stock_quantity = MAX(0, stock_quantity - ?), updated_at = datetime('now') WHERE id = ? AND track_inventory = 1`,
			l.qty, l.itemID,
		); err != nil {
			return nil, err
		}
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE orders SET status = 'paid', is_edit = 0, updated_at = datetime('now') WHERE id = ?`, orderID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getOrder(ctx, db, orderID)
}

// ClearOrder deletes an unpaid order (cashier starts over).
func ClearOrder(ctx context.Context, db *sql.DB, orderID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM orders WHERE id = ? AND status != 'paid' AND is_edit = 0`, orderID)
	return err
}

// ensureOpenOrder returns the current open/displayed order for terminal T1,
// creating it with the next sequence number when none exists.
func ensureOpenOrder(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var orderID string
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM orders WHERE status IN ('open','displayed') ORDER BY updated_at DESC, created_at DESC LIMIT 1`).Scan(&orderID)
	if err == nil {
		return orderID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var seq int64
	if err := tx.QueryRowContext(ctx,
		`UPDATE sequences SET value = value + 1 WHERE name = 'T1' RETURNING value`).Scan(&seq); err != nil {
		return "", err
	}
	orderID = NewID()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO orders (id, order_no, user_id) VALUES (?, ?, ?)`,
		orderID, fmt.Sprintf("T1-%04d", seq), userID)
	return orderID, err
}

// recalcTotals recomputes subtotal and VAT for the order inside its transaction.
func recalcTotals(ctx context.Context, tx *sql.Tx, orderID string) error {
	var subtotal int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(line_total_kobo),0) FROM order_lines WHERE order_id = ?`, orderID).Scan(&subtotal); err != nil {
		return err
	}
	var s Settings
	var vatEnabled int
	err := tx.QueryRowContext(ctx,
		`SELECT business_name, receipt_footer, vat_enabled, vat_rate_bp, vat_mode, packaging_fee_kobo FROM settings WHERE id = 1`,
	).Scan(&s.BusinessName, &s.ReceiptFooter, &vatEnabled, &s.VATRateBP, &s.VATMode, &s.PackagingFeeKobo)
	if err != nil {
		return err
	}
	s.VATEnabled = vatEnabled == 1
	_, vat, total := core.ApplyVAT(subtotal, core.VATSettings{Enabled: s.VATEnabled, RateBP: s.VATRateBP, Mode: s.VATMode})
	_, err = tx.ExecContext(ctx,
		`UPDATE orders SET subtotal_kobo = ?, vat_kobo = ?, total_kobo = ?, updated_at = datetime('now') WHERE id = ?`,
		subtotal, vat, total, orderID)
	return err
}
