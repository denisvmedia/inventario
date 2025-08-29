package postgres

import (
	"github.com/google/uuid"
)

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}
