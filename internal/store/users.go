package store

import (
	"context"
	"database/sql"
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
