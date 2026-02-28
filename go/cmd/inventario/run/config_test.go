package run

import "testing"

func TestConfigSetDefaults_PreservesExplicitZeroEmailQueueMaxRetries(t *testing.T) {
	cfg := Config{
		EmailQueueMaxRetries: 0,
	}

	cfg.setDefaults()

	if cfg.EmailQueueMaxRetries != 0 {
		t.Fatalf("expected EmailQueueMaxRetries to remain 0, got %d", cfg.EmailQueueMaxRetries)
	}
}

func TestConfigSetDefaults_DefaultsNegativeEmailQueueMaxRetries(t *testing.T) {
	cfg := Config{
		EmailQueueMaxRetries: -1,
	}

	cfg.setDefaults()

	if cfg.EmailQueueMaxRetries != 5 {
		t.Fatalf("expected EmailQueueMaxRetries to default to 5, got %d", cfg.EmailQueueMaxRetries)
	}
}
