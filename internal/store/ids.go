package store

import (
	"github.com/google/uuid"
)

func NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err) // crypto/rand failure is unrecoverable
	}
	return id.String()
}
