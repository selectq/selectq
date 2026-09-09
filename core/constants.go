package core

const (
	// HDFC statement layout constants (1-indexed row numbers in XLSX)
	headerRow    = 21 // "Date | Narration | Chq./Ref.No. | ..."
	separatorRow = 22 // "******** | ******* | ..."
	DataStartRow = 23 // First actual transaction row
)
