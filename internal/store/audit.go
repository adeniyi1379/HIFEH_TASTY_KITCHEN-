package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// AuditEntry is one immutable line of "who did what, where, when".
type AuditEntry struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	UserName   string `json:"user_name"`
	TerminalID string `json:"terminal_id"`
	Action     string `json:"action"`
	Entity     string `json:"entity"`
	EntityID   string `json:"entity_id"`
	Details    string `json:"details"`
	At         string `json:"at"`
}

// Audit appends an entry. It never updates or deletes anything.
// user_name is snapshotted so history survives later user edits.
func Audit(ctx context.Context, db *sql.DB, userID, userName, terminalID, action, entity, entityID string, details map[string]any) error {
	body := "{}"
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return err
		}
		body = string(b)
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO audit_log (id, user_id, user_name, terminal_id, action, entity, entity_id, details)
		 VALUES (?,?,?,?,?,?,?,?)`,
		NewID(), userID, userName, terminalID, action, entity, entityID, body)
	return err
}

func ListAudit(ctx context.Context, db *sql.DB, limit int) ([]AuditEntry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, COALESCE(user_id,''), COALESCE(user_name,''), terminal_id, action, entity, entity_id, details, at
		 FROM audit_log ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.UserName, &e.TerminalID, &e.Action, &e.Entity, &e.EntityID, &e.Details, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
