package apiserver

// The login length caps, exposed so a test can pin the numbers against the
// reasons stated for them. They compile only under `go test`.
const (
	LoginEmailMaxLenForTest    = loginEmailMaxLen
	LoginPasswordMaxLenForTest = loginPasswordMaxLen
)
