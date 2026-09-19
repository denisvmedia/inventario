package services

import (
	"go.5x5.cz/inventario/models"
)

// LoadConcurrentUploadConfig loads concurrent upload configuration from environment variables
func LoadConcurrentUploadConfig() models.SlotManagerConfig {
	return models.LoadSlotManagerConfigFromEnv()
}
