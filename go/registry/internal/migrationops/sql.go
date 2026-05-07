package migrationops

import "database/sql"

// sqlNoRows is aliased here so the rest of the package stays free of
// the database/sql import.
var sqlNoRows = sql.ErrNoRows
