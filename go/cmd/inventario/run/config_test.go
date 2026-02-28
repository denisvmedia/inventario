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

func TestValidatePublicURLForTransactionalEmails(t *testing.T) {
	cases := []struct {
		name      string
		publicURL string
		wantErr   bool
	}{
		{name: "valid https", publicURL: "https://inventario.example.com", wantErr: false},
		{name: "missing", publicURL: "", wantErr: true},
		{name: "missing scheme", publicURL: "inventario.example.com", wantErr: true},
		{name: "unsupported scheme", publicURL: "ftp://inventario.example.com", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePublicURLForTransactionalEmails(tc.publicURL)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.publicURL)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.publicURL, err)
			}
		})
	}
}
