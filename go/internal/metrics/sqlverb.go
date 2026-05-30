package metrics

// parseSQLVerb extracts a bounded, lowercase operation label from the
// first keyword of a SQL statement. The returned value is always one
// of a fixed set, so it is safe to use as a Prometheus label without
// risking cardinality blow-up:
//
//	select, insert, update, delete, begin, commit, rollback, with, other
//
// Leading whitespace, SQL line comments ("-- ..."), and leading "("
// are skipped. Only the first token is lowercased (not the whole
// string) to keep the hot path allocation-light. Anything outside the
// known set maps to "other" (e.g. "SET LOCAL ROLE ...").
func parseSQLVerb(sql string) string {
	i := skipLeading(sql)

	// Capture the first token: a run of ASCII letters.
	start := i
	for i < len(sql) && isASCIILetter(sql[i]) {
		i++
	}
	if i == start {
		return verbOther
	}

	// Match the token case-insensitively against the known verbs
	// without lowercasing the whole string. lowerToken allocates at
	// most the small first word.
	switch lowerToken(sql[start:i]) {
	case verbSelect:
		return verbSelect
	case verbInsert:
		return verbInsert
	case verbUpdate:
		return verbUpdate
	case verbDelete:
		return verbDelete
	case verbBegin:
		return verbBegin
	case verbCommit:
		return verbCommit
	case verbRollback:
		return verbRollback
	case verbWith:
		return verbWith
	default:
		return verbOther
	}
}

const (
	verbSelect   = "select"
	verbInsert   = "insert"
	verbUpdate   = "update"
	verbDelete   = "delete"
	verbBegin    = "begin"
	verbCommit   = "commit"
	verbRollback = "rollback"
	verbWith     = "with"
	verbOther    = "other"
)

// skipLeading returns the index of the first byte of the first SQL
// keyword, skipping whitespace, "-- ..." line comments, and "(".
func skipLeading(sql string) int {
	i := 0
	for i < len(sql) {
		c := sql[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			i++
		case c == '-' && i+1 < len(sql) && sql[i+1] == '-':
			// Line comment: skip to end of line.
			i += 2
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
		default:
			return i
		}
	}
	return i
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// lowerToken returns the ASCII-lowercased form of a short token. It
// avoids allocating when the token is already lowercase.
func lowerToken(tok string) string {
	needsLower := false
	for i := 0; i < len(tok); i++ {
		if tok[i] >= 'A' && tok[i] <= 'Z' {
			needsLower = true
			break
		}
	}
	if !needsLower {
		return tok
	}
	b := make([]byte, len(tok))
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
