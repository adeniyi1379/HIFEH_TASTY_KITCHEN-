package store

import (
	"context"
	"database/sql"
)

type CashierSummary struct {
	UserID     string `json:"user_id"`
	UserName   string `json:"user_name"`
	OrderCount int    `json:"order_count"`
	TotalKobo  int64  `json:"total_kobo"`
}

type ItemSalesSummary struct {
	ItemID    string `json:"item_id"`
	ItemName  string `json:"item_name"`
	Quantity  int64  `json:"quantity"`
	TotalKobo int64  `json:"total_kobo"`
}

type RecentOrderSummary struct {
	ID        string  `json:"id"`
	OrderNo   string  `json:"order_no"`
	Status    string  `json:"status"`
	Type      string  `json:"type"`
	TableNo   *string `json:"table_no"`
	TotalKobo int64   `json:"total_kobo"`
	UserName  string  `json:"user_name"`
	CreatedAt string  `json:"created_at"`
}

type TodayReport struct {
	Date         string               `json:"date"`
	PaidCount    int                  `json:"paid_count"`
	TotalSales   int64                `json:"total_sales_kobo"`
	Subtotal     int64                `json:"subtotal_kobo"`
	TotalVAT     int64                `json:"vat_kobo"`
	ByCashier    []CashierSummary     `json:"by_cashier"`
	TopItems     []ItemSalesSummary   `json:"top_items"`
	RecentOrders []RecentOrderSummary `json:"recent_orders"`
}

func GetTodayReport(ctx context.Context, db *sql.DB) (TodayReport, error) {
	var r TodayReport

	err := db.QueryRowContext(ctx, `SELECT date('now')`).Scan(&r.Date)
	if err != nil {
		return r, err
	}

	// Overall paid orders today
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(subtotal_kobo), 0), COALESCE(SUM(vat_kobo), 0), COALESCE(SUM(total_kobo), 0)
		 FROM orders
		 WHERE status = 'paid' AND date(created_at) = date('now')`,
	).Scan(&r.PaidCount, &r.Subtotal, &r.TotalVAT, &r.TotalSales)
	if err != nil {
		return r, err
	}

	// By cashier
	cRows, err := db.QueryContext(ctx,
		`SELECT o.user_id, COALESCE(u.name, 'Unknown'), COUNT(o.id), COALESCE(SUM(o.total_kobo), 0)
		 FROM orders o
		 LEFT JOIN users u ON u.id = o.user_id
		 WHERE o.status = 'paid' AND date(o.created_at) = date('now')
		 GROUP BY o.user_id, u.name
		 ORDER BY SUM(o.total_kobo) DESC`,
	)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var cs CashierSummary
			if err := cRows.Scan(&cs.UserID, &cs.UserName, &cs.OrderCount, &cs.TotalKobo); err == nil {
				r.ByCashier = append(r.ByCashier, cs)
			}
		}
	}
	if r.ByCashier == nil {
		r.ByCashier = []CashierSummary{}
	}

	// Top items sold today
	iRows, err := db.QueryContext(ctx,
		`SELECT ol.item_id, ol.name_snapshot, COALESCE(SUM(ol.quantity), 0), COALESCE(SUM(ol.line_total_kobo), 0)
		 FROM order_lines ol
		 JOIN orders o ON o.id = ol.order_id
		 WHERE o.status = 'paid' AND date(o.created_at) = date('now')
		 GROUP BY ol.item_id, ol.name_snapshot
		 ORDER BY SUM(ol.quantity) DESC
		 LIMIT 10`,
	)
	if err == nil {
		defer iRows.Close()
		for iRows.Next() {
			var is ItemSalesSummary
			if err := iRows.Scan(&is.ItemID, &is.ItemName, &is.Quantity, &is.TotalKobo); err == nil {
				r.TopItems = append(r.TopItems, is)
			}
		}
	}
	if r.TopItems == nil {
		r.TopItems = []ItemSalesSummary{}
	}

	// Recent 20 orders
	oRows, err := db.QueryContext(ctx,
		`SELECT o.id, o.order_no, o.status, o.type, o.table_no, o.total_kobo, COALESCE(u.name, '—'), o.created_at
		 FROM orders o
		 LEFT JOIN users u ON u.id = o.user_id
		 ORDER BY o.created_at DESC
		 LIMIT 20`,
	)
	if err == nil {
		defer oRows.Close()
		for oRows.Next() {
			var ro RecentOrderSummary
			var tableNo sql.NullString
			if err := oRows.Scan(&ro.ID, &ro.OrderNo, &ro.Status, &ro.Type, &tableNo, &ro.TotalKobo, &ro.UserName, &ro.CreatedAt); err == nil {
				if tableNo.Valid {
					ro.TableNo = &tableNo.String
				}
				r.RecentOrders = append(r.RecentOrders, ro)
			}
		}
	}
	if r.RecentOrders == nil {
		r.RecentOrders = []RecentOrderSummary{}
	}

	return r, nil
}
