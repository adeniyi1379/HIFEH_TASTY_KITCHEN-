package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Active bool   `json:"active"`
}

func FindByPIN(ctx context.Context, db *sql.DB, pin string) (*User, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, role, pin_hash, active FROM users WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, role, hash string
		var active int
		if err := rows.Scan(&id, &name, &role, &hash, &active); err != nil {
			return nil, err
		}
		// Constant-time compare per user; staff list is small.
		if VerifyPIN(hash, pin) {
			return &User{ID: id, Name: name, Role: role, Active: active == 1}, nil
		}
	}
	return nil, rows.Err()
}

// GetUserByID loads one user, including inactive ones.
func GetUserByID(ctx context.Context, db *sql.DB, id string) (*User, error) {
	var u User
	var active int
	err := db.QueryRowContext(ctx,
		`SELECT id, name, role, active FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Name, &u.Role, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Active = active == 1
	return &u, nil
}

func ListUsers(ctx context.Context, db *sql.DB) ([]User, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, name, role, active FROM users ORDER BY role, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var active int
		if err := rows.Scan(&u.ID, &u.Name, &u.Role, &active); err != nil {
			return nil, err
		}
		u.Active = active == 1
		out = append(out, u)
	}
	return out, rows.Err()
}

func CreateUser(ctx context.Context, db *sql.DB, name, role, pin string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return User{}, errors.New("name cannot be empty")
	}
	if role != "cashier" && role != "manager" && role != "owner" {
		return User{}, errors.New("invalid role; must be cashier, manager, or owner")
	}
	pin = strings.TrimSpace(pin)
	if len(pin) < 4 || len(pin) > 8 {
		return User{}, errors.New("PIN must be 4 to 8 digits")
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return User{}, errors.New("PIN must contain only numbers")
		}
	}

	hash, err := HashPIN(pin)
	if err != nil {
		return User{}, err
	}

	id := NewID()
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, name, role, pin_hash, active) VALUES (?, ?, ?, ?, 1)`,
		id, name, role, hash,
	)
	if err != nil {
		return User{}, err
	}
	return User{ID: id, Name: name, Role: role, Active: true}, nil
}

func UpdateUser(ctx context.Context, db *sql.DB, id, name, role string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if role != "cashier" && role != "manager" && role != "owner" {
		return errors.New("invalid role")
	}
	res, err := db.ExecContext(ctx,
		`UPDATE users SET name = ?, role = ?, updated_at = datetime('now') WHERE id = ?`,
		name, role, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

func SetUserActive(ctx context.Context, db *sql.DB, id string, active bool) error {
	act := 0
	if active {
		act = 1
	}
	res, err := db.ExecContext(ctx,
		`UPDATE users SET active = ?, updated_at = datetime('now') WHERE id = ?`,
		act, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

func ResetPIN(ctx context.Context, db *sql.DB, id, newPIN string) error {
	newPIN = strings.TrimSpace(newPIN)
	if len(newPIN) < 4 || len(newPIN) > 8 {
		return errors.New("PIN must be 4 to 8 digits")
	}
	for _, c := range newPIN {
		if c < '0' || c > '9' {
			return errors.New("PIN must contain only numbers")
		}
	}

	hash, err := HashPIN(newPIN)
	if err != nil {
		return err
	}

	res, err := db.ExecContext(ctx,
		`UPDATE users SET pin_hash = ?, updated_at = datetime('now') WHERE id = ?`,
		hash, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

func DeleteUser(ctx context.Context, db *sql.DB, id string) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders WHERE user_id = ?`, id).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("cannot delete worker with recorded sales history; deactivate instead")
	}
	res, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}
