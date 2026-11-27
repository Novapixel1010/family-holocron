// Package gendate parses the loose date formats genealogists use.
//
// Accepted inputs (case-insensitive):
//
//	1850            1850-03        1850-03-12
//	12 MAR 1850     MAR 1850       ABT 1850 / EST / CAL / BEF / AFT / CIRCA / C.
//	BET 1850 AND 1860              FROM 1850 TO 1860
//
// Anything else is kept verbatim but has no sort key (it shows up as "undated").
package gendate

import (
	"fmt"
	"strconv"
	"strings"
)

// Date is a parsed genealogical date.
type Date struct {
	Raw       string // the original input, trimmed
	Qualifier string // "", "ABT", "BEF", "AFT", "BET", "FROM", "EST", "CAL"
	Year      int    // 0 when unknown
	Month     int    // 0 when unknown
	Day       int    // 0 when unknown
}

// Valid reports whether the date has at least a year.
func (d Date) Valid() bool { return d.Year > 0 }

// SortKey returns a lexicographically sortable "YYYY-MM-DD" key (unknown parts are 00),
// or "" when the date could not be parsed.
func (d Date) SortKey() string {
	if !d.Valid() {
		return ""
	}
	key := fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	switch d.Qualifier {
	case "BEF":
		// "before 1850" sorts after everything in 1849 but ahead of 1850 itself.
		switch {
		case d.Month == 0:
			key = fmt.Sprintf("%04d-14-00", d.Year-1)
		case d.Day == 0:
			key = fmt.Sprintf("%04d-%02d-00", d.Year, d.Month-1) + "~"
		}
	case "AFT":
		// "after 1850" sorts after every dated entry within 1850.
		switch {
		case d.Month == 0:
			key = fmt.Sprintf("%04d-13-00", d.Year)
		case d.Day == 0:
			key = fmt.Sprintf("%04d-%02d-32", d.Year, d.Month)
		default:
			key += "~"
		}
	}
	return key
}

var months = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
	"JANUARY": 1, "FEBRUARY": 2, "MARCH": 3, "APRIL": 4, "JUNE": 6, "JULY": 7,
	"AUGUST": 8, "SEPT": 9, "SEPTEMBER": 9, "OCTOBER": 10, "NOVEMBER": 11, "DECEMBER": 12,
}

var monthAbbr = []string{"", "JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}

var qualifiers = map[string]string{
	"ABT": "ABT", "ABOUT": "ABT", "CIRCA": "ABT", "C.": "ABT", "CA.": "ABT", "CA": "ABT", "~": "ABT",
	"EST": "EST", "CAL": "CAL", "BEF": "BEF", "BEFORE": "BEF", "AFT": "AFT", "AFTER": "AFT",
	"BET": "BET", "BETWEEN": "BET", "FROM": "FROM", "TO": "BEF", "INT": "",
}

// Parse parses s. It never fails: unparseable input yields a Date with Year == 0.
func Parse(s string) Date {
	raw := strings.TrimSpace(s)
	d := Date{Raw: raw}
	if raw == "" {
		return d
	}
	fields := strings.Fields(strings.ToUpper(strings.ReplaceAll(raw, ",", " ")))
	if q, ok := qualifiers[fields[0]]; ok {
		d.Qualifier = q
		fields = fields[1:]
	}
	// For ranges, the first bound determines sorting.
	for i, f := range fields {
		if f == "AND" || f == "TO" || f == "-" {
			fields = fields[:i]
			break
		}
	}
	y, m, dd, ok := parseCore(fields)
	if !ok {
		return Date{Raw: raw}
	}
	d.Year, d.Month, d.Day = y, m, dd
	return d
}

func parseCore(f []string) (y, m, d int, ok bool) {
	switch len(f) {
	case 1:
		// ISO forms.
		parts := strings.Split(f[0], "-")
		if len(parts) > 3 {
			return 0, 0, 0, false
		}
		var nums [3]int
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil {
				return 0, 0, 0, false
			}
			nums[i] = n
		}
		y, m, d = nums[0], nums[1], nums[2]
		if len(parts[0]) < 3 {
			return 0, 0, 0, false
		}
	case 2: // MAR 1850
		mm, okm := months[f[0]]
		yy, err := strconv.Atoi(f[1])
		if !okm || err != nil {
			return 0, 0, 0, false
		}
		y, m = yy, mm
	case 3: // 12 MAR 1850 or MAR 12 1850
		dd, err1 := strconv.Atoi(f[0])
		mm, okm := months[f[1]]
		if err1 != nil || !okm {
			mm, okm = months[f[0]]
			dd, err1 = strconv.Atoi(f[1])
			if err1 != nil || !okm {
				return 0, 0, 0, false
			}
		}
		yy, err := strconv.Atoi(f[2])
		if err != nil {
			return 0, 0, 0, false
		}
		y, m, d = yy, mm, dd
	default:
		return 0, 0, 0, false
	}
	if y < 1 || y > 9999 || m < 0 || m > 12 || d < 0 || d > 31 || (m == 0 && d != 0) {
		return 0, 0, 0, false
	}
	return y, m, d, true
}

// GEDCOM renders the date in GEDCOM 5.5.1 form ("ABT 12 MAR 1850").
// Unparseable dates are returned verbatim (GEDCOM permits date phrases).
func (d Date) GEDCOM() string {
	if !d.Valid() {
		if d.Raw == "" {
			return ""
		}
		return "(" + d.Raw + ")"
	}
	// Ranges are kept as typed when they already look like GEDCOM.
	up := strings.ToUpper(d.Raw)
	if d.Qualifier == "BET" || d.Qualifier == "FROM" {
		return up
	}
	var b strings.Builder
	if d.Qualifier != "" {
		b.WriteString(d.Qualifier + " ")
	}
	if d.Day > 0 {
		fmt.Fprintf(&b, "%d ", d.Day)
	}
	if d.Month > 0 {
		b.WriteString(monthAbbr[d.Month] + " ")
	}
	fmt.Fprintf(&b, "%d", d.Year)
	return b.String()
}

// FromGEDCOM converts a GEDCOM date value into the form stored by Holocron.
// Plain exact dates become ISO ("1850-03-12"); qualified dates keep GEDCOM syntax.
func FromGEDCOM(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	d := Parse(s)
	if !d.Valid() || d.Qualifier != "" {
		return s
	}
	switch {
	case d.Day > 0:
		return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
	case d.Month > 0:
		return fmt.Sprintf("%04d-%02d", d.Year, d.Month)
	default:
		return fmt.Sprintf("%04d", d.Year)
	}
}
