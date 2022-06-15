package util

// PageSize represents the WASM page size in bytes.
const PageSize = 65535

// Pages converts a byte size to Pages, rounding up as necessary.
func Pages(n uint32) uint32 {
	pages := n / PageSize
	if pages*PageSize == n {
		return pages
	}

	return pages + 1
}
