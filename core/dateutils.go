package core

import (
	"fmt"
	"time"
)

// AddDaysToDate adds the specified number of days to a YYYY-MM-DD date string.
// It avoids limitations of standard AddDate by adding absolute duration.
func AddDaysToDate(dateStr string, days int) (string, error) {
	if dateStr == "" {
		return "", nil
	}
	
	// Parse the date assuming YYYY-MM-DD
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", fmt.Errorf("invalid date format: %w", err)
	}
	
	// We add exactly 24 hour intervals to the UTC time to avoid any timezone or library edge cases
	newTime := t.UTC().Add(time.Duration(days) * 24 * time.Hour)
	
	return newTime.Format("2006-01-02"), nil
}
