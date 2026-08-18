package utils

import "strconv"

// FormatIDR formats a whole-number amount as Indonesian Rupiah with dots
// as thousands separators, e.g. 1500000 -> "Rp1.500.000".
// IDR has no subunit in everyday use, so this only handles integers.
func FormatIDR(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	digits := strconv.FormatInt(amount, 10)

	n := len(digits)
	firstGroup := n % 3
	if firstGroup == 0 {
		firstGroup = 3
	}

	grouped := digits[:firstGroup]
	for i := firstGroup; i < n; i += 3 {
		grouped += "." + digits[i:i+3]
	}

	if neg {
		return "-Rp" + grouped
	}
	return "Rp" + grouped
}
