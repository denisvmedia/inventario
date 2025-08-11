package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-extras/cobraflags"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/errkit"
	ptahintegration "github.com/denisvmedia/inventario/registry/ptah"
)

const (
	operationalUserFlag = "operational-user"
)

var flags = map[string]cobraflags.Flag{
	operationalUserFlag: &cobraflags.StringFlag{
		Name:  operationalUserFlag,
		Usage: "Operational user to set as owner of database objects",
	},
}

// CreatePtahMigrator creates a Ptah migrator instance
func CreatePtahMigrator(dsn string) (*ptahintegration.PtahMigrator, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("Ptah migrations only support PostgreSQL databases")
	}

	// Create the migrator with the models directory for schema parsing
	migrator, err := ptahintegration.NewPtahMigrator(dsn, "./models")
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create Ptah migrator")
	}

	return migrator, nil
}

func RegisterOperationalUserFlag(cmd *cobra.Command) {
	cobraflags.RegisterMap(cmd, flags)
}

func GetOperationalUser(opUser, dsn string) string {
	if opUser == "" {
		// extract user from dsn
		u, err := url.Parse(dsn)
		if err != nil {
			return ""
		}
		opUser = u.User.Username()
	}
	return opUser
}
