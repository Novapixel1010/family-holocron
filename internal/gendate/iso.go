package gendate

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Birth and death dates use ISO 8601 calendar dates: YYYY-MM-DD, or the partial
// forms YYYY-MM and YYYY when only part of the date is known.

var isoRe = regexp.MustCompile(`^(\d{4})(?:-(\d{1,2})(?:-(\d{1,2}))?)?$`)

// ErrISODate describes the accepted format.
var ErrISODate = fmt.Errorf("use YYYY-MM-DD (e.g. 1890-03-12), or YYYY-MM / YYYY if the day or month is unknown")

// NormalizeISO validates s as YYYY-MM-DD, YYYY-MM or YYYY and returns it zero-padded
// ("1890-3-5" → "1890-03-05"). Impossible calendar dates (e.g. 1900-02-29) are rejected.
func NormalizeISO(s string) (string, error) {
	m := isoRe.FindStringSubmatch(s)
	if m == nil {
		return "", ErrISODate
	}
	y, _ := strconv.Atoi(m[1])
	if y < 1 {
		return "", fmt.Errorf("year must be 0001 or later")
	}
	if m[2] == "" {
		return fmt.Sprintf("%04d", y), nil
	}
	mo, _ := strconv.Atoi(m[2])
	if mo < 1 || mo > 12 {
		return "", fmt.Errorf("month must be between 01 and 12")
	}
	if m[3] == "" {
		return fmt.Sprintf("%04d-%02d", y, mo), nil
	}
	d, _ := strconv.Atoi(m[3])
	if d < 1 || d > daysIn(y, mo) {
		return "", fmt.Errorf("%04d-%02d has no day %d", y, mo, d)
	}
	return fmt.Sprintf("%04d-%02d-%02d", y, mo, d), nil
}

func daysIn(y, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// IsISO reports whether s is already a normalized YYYY[-MM[-DD]] value.
func IsISO(s string) bool {
	n, err := NormalizeISO(s)
	return err == nil && n == s
}

// CompareISO compares two normalized ISO dates at their shared precision:
// "1850" vs "1850-06-01" compares equal; "1849-12" vs "1850" is before.
func CompareISO(a, b string) int {
	n := min(len(a), len(b))
	switch {
	case a[:n] < b[:n]:
		return -1
	case a[:n] > b[:n]:
		return 1
	}
	return 0
}

// AfterToday reports whether the date lies in the future at its own precision
// (a year counts as future only when the whole year is). The comparison allows
// a day of slack so "today" in the earliest time zone is never rejected.
func AfterToday(iso string, now time.Time) bool {
	today := now.UTC().Add(24 * time.Hour).Format("2006-01-02")
	return iso[:min(len(iso), len(today))] > today[:min(len(iso), len(today))]
}
