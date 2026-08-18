package utils

import "fmt"

func FormatDuration(totalMinutes int) string {
	if totalMinutes < 0 {
		totalMinutes = 0
	}
	hours := totalMinutes / 60
	minutes := totalMinutes % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
