package types

// ExportStats tracks statistics during export generation
type ExportStats struct {
	LocationCount  int
	AreaCount      int
	CommodityCount int
	ImageCount     int
	InvoiceCount   int
	ManualCount    int
	BinaryDataSize int64
}
