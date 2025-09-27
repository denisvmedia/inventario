package services

import (
	"github.com/denisvmedia/inventario/models"
)

// LoadConcurrentUploadConfig loads concurrent upload configuration from environment variables
func LoadConcurrentUploadConfig() models.SlotManagerConfig {
	return models.LoadSlotManagerConfigFromEnv()
}
