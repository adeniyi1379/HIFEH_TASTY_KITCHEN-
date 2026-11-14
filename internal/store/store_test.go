package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Seed(ctx, db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

// testUser creates a cashier and returns their ID, for order attribution.
func testUser(t *testing.T, db *sql.DB) string {
	t.Helper()
	u, err := CreateUser(context.Background(), db, "Test Cashier", "cashier", "9999")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}

func createTestItem(t *testing.T, db *sql.DB, name string, priceKobo int64, stock int64) Item {
	t.Helper()
	it, err := CreateItem(context.Background(), db, "cat_drinks", name, "count", "piece", priceKobo, true, stock, false, nil)
	if err != nil {
		t.Fatalf("create item %s: %v", name, err)
	}
	return it
}

func createTestCombo(t *testing.T, db *sql.DB, priceKobo int64, comps []ComboComponentInput) Item {
	t.Helper()
	it, err := CreateItem(context.Background(), db, "cat_drinks", "Test Combo Meal", "count", "combo", priceKobo, false, 0, true, comps)
	if err != nil {
		t.Fatalf("create combo: %v", err)
	}
	return it
}

func TestAddComboExpandsComponents(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 10)
	dodo := createTestItem(t, db, "Test Dodo", 10000, 10)
	combo := createTestCombo(t, db, 35000, []ComboComponentInput{
		{ItemID: coke.ID, Quantity: 1},
		{ItemID: dodo.ID, Quantity: 2},
	})

	o, err := AddItem(ctx, db, combo.ID, testUser(t, db))
	if err != nil {
		t.Fatalf("add combo: %v", err)
	}
	if len(o.Lines) != 3 {
		t.Fatalf("expected 3 lines (combo + 2 components), got %d", len(o.Lines))
	}
	var comboLine, freeLine OrderLine
	for _, l := range o.Lines {
		switch l.ItemID {
		case combo.ID:
			comboLine = l
		case dodo.ID:
			freeLine = l
		}
	}
	if comboLine.Quantity != 1 || comboLine.LineTotalKobo != 35000 {
		t.Errorf("combo line wrong: %+v", comboLine)
	}
	if freeLine.Quantity != 2 || freeLine.UnitPriceKobo != 0 || freeLine.LineTotalKobo != 0 {
		t.Errorf("component line wrong: %+v", freeLine)
	}
	if o.SubtotalKobo != 35000 || o.TotalKobo != 35000 {
		t.Errorf("totals wrong: subtotal=%d total=%d", o.SubtotalKobo, o.TotalKobo)
	}
}

func TestComboStockCheckedAndDeducted(t *testing.T) {
	db := testDB(t)
	uid := testUser(t, db)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 1)
	combo := createTestCombo(t, db, 35000, []ComboComponentInput{
		{ItemID: coke.ID, Quantity: 1},
	})

	// Adding the combo a second time should fail: only 1 Coke in stock.
	if _, err := AddItem(ctx, db, combo.ID, uid); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := AddItem(ctx, db, combo.ID, uid); err == nil {
		t.Fatal("second add should fail on component stock")
	}

	ord, err := GetLatestOrder(ctx, db)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if _, err := PayOrder(ctx, db, ord.ID); err != nil {
		t.Fatalf("pay: %v", err)
	}
	stock, err := GetItemStock(ctx, db, coke.ID)
	if err != nil {
		t.Fatalf("get stock: %v", err)
	}
	if stock != 0 {
		t.Errorf("coke stock after pay = %d, want 0", stock)
	}

	// Void restores component stock.
	if _, err := VoidOrder(ctx, db, ord.ID); err != nil {
		t.Fatalf("void: %v", err)
	}
	stock, _ = GetItemStock(ctx, db, coke.ID)
	if stock != 1 {
		t.Errorf("coke stock after void = %d, want 1", stock)
	}
}

func TestComboQtyChangePropagates(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 10)
	combo := createTestCombo(t, db, 35000, []ComboComponentInput{
		{ItemID: coke.ID, Quantity: 1},
	})

	o, _ := AddItem(ctx, db, combo.ID, testUser(t, db))
	o, _ = AddItem(ctx, db, combo.ID, testUser(t, db))
	var comboLineID string
	for _, l := range o.Lines {
		if l.ItemID == combo.ID {
			comboLineID = l.ID
		}
	}

	o, err := ChangeLineQty(ctx, db, comboLineID, -1)
	if err != nil {
		t.Fatalf("dec combo: %v", err)
	}
	// After qty 2 -> 1: combo line + one free Coke line remain.
	var comboQty, cokeQty int64
	for _, l := range o.Lines {
		if l.ItemID == combo.ID {
			comboQty = l.Quantity
		}
		if l.ItemID == coke.ID {
			cokeQty += l.Quantity
		}
	}
	if comboQty != 1 || cokeQty != 1 {
		t.Errorf("after dec: comboQty=%d cokeQty=%d, want 1 and 1", comboQty, cokeQty)
	}

	o, err = ChangeLineQty(ctx, db, comboLineID, 1)
	if err != nil {
		t.Fatalf("inc combo: %v", err)
	}
	cokeQty = 0
	for _, l := range o.Lines {
		if l.ItemID == coke.ID {
			cokeQty += l.Quantity
		}
	}
	if cokeQty != 2 {
		t.Errorf("coke qty after inc = %d, want 2", cokeQty)
	}
	if o.SubtotalKobo != 70000 {
		t.Errorf("subtotal = %d, want 70000", o.SubtotalKobo)
	}
}

func TestCannotEditPaidOrder(t *testing.T) {
	db := testDB(t)
	uid := testUser(t, db)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 10)
	if _, err := AddItem(ctx, db, coke.ID, uid); err != nil {
		t.Fatalf("add: %v", err)
	}
	ord, err := GetLatestOrder(ctx, db)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if _, err := PayOrder(ctx, db, ord.ID); err != nil {
		t.Fatalf("pay: %v", err)
	}

	if _, err := ChangeLineQty(ctx, db, ord.Lines[0].ID, 1); err == nil {
		t.Error("ChangeLineQty on paid order should fail")
	}
	if _, err := ChangeLineQty(ctx, db, ord.Lines[0].ID, -1); err == nil {
		t.Error("ChangeLineQty(-1) on paid order should fail")
	}
	if _, err := RemoveLine(ctx, db, ord.Lines[0].ID); err == nil {
		t.Error("RemoveLine on paid order should fail")
	}
}

func TestStandaloneAndFreeLinesDoNotMerge(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 10)
	combo := createTestCombo(t, db, 35000, []ComboComponentInput{
		{ItemID: coke.ID, Quantity: 1},
	})

	// Cashier adds Coke standalone, then the combo with a free Coke.
	o, _ := AddItem(ctx, db, coke.ID, testUser(t, db))
	o, err := AddItem(ctx, db, combo.ID, testUser(t, db))
	if err != nil {
		t.Fatalf("add combo: %v", err)
	}
	if len(o.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %+v", len(o.Lines), o.Lines)
	}
	// Subtotal = 30000 (paid Coke) + 35000 (combo). Free Coke adds nothing.
	if o.SubtotalKobo != 65000 {
		t.Errorf("subtotal = %d, want 65000", o.SubtotalKobo)
	}
}

func TestComboValidation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 10)

	if _, err := CreateItem(ctx, db, "cat_drinks", "Empty Combo", "count", "combo", 35000, false, 0, true, nil); err == nil {
		t.Error("combo without components should fail")
	}
	if _, err := CreateItem(ctx, db, "cat_drinks", "Ghost Combo", "count", "combo", 35000, false, 0, true, []ComboComponentInput{{ItemID: "nope"}}); err == nil {
		t.Error("combo with unknown component should fail")
	}

	// Combo-in-combo should fail.
	combo := createTestCombo(t, db, 35000, []ComboComponentInput{{ItemID: coke.ID, Quantity: 1}})
	if _, err := CreateItem(ctx, db, "cat_drinks", "Nested Combo", "count", "combo", 35000, false, 0, true, []ComboComponentInput{{ItemID: combo.ID, Quantity: 1}}); err == nil {
		t.Error("combo containing a combo should fail")
	}

	// Component item cannot be deleted while used in a combo.
	if err := DeleteItem(ctx, db, coke.ID); err == nil {
		t.Error("deleting component of a combo should fail")
	}
}

func TestSetOrderMetaRejectsBadType(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 10)
	AddItem(ctx, db, coke.ID, testUser(t, db))
	ord, err := GetLatestOrder(ctx, db)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if _, err := SetOrderMeta(ctx, db, ord.ID, "yolo", ""); err == nil {
		t.Error("invalid order type should fail")
	}
}

func TestListRecentOrdersNoNPlusOne(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	coke := createTestItem(t, db, "Test Coke", 30000, 100)
	for i := 0; i < 3; i++ {
		AddItem(ctx, db, coke.ID, testUser(t, db))
		ord, err := GetLatestOrder(ctx, db)
		if err != nil {
			t.Fatalf("latest: %v", err)
		}
		if _, err := PayOrder(ctx, db, ord.ID); err != nil {
			t.Fatalf("pay: %v", err)
		}
	}
	orders, err := ListRecentOrders(ctx, db, 20)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}
	for _, o := range orders {
		if len(o.Lines) != 1 {
			t.Errorf("order %s has %d lines, want 1", o.OrderNo, len(o.Lines))
		}
	}
}
