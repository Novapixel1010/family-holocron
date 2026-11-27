package gendate

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in, sort, ged string
	}{
		{"1850", "1850-00-00", "1850"},
		{"1850-03", "1850-03-00", "MAR 1850"},
		{"1850-03-12", "1850-03-12", "12 MAR 1850"},
		{"12 MAR 1850", "1850-03-12", "12 MAR 1850"},
		{"abt 1850", "1850-00-00", "ABT 1850"},
		{"Circa 1850", "1850-00-00", "ABT 1850"},
		{"BEF 1900", "1899-14-00", "BEF 1900"},
		{"AFT 1900", "1900-13-00", "AFT 1900"},
		{"BET 1850 AND 1860", "1850-00-00", "BET 1850 AND 1860"},
		{"March 3, 1901", "1901-03-03", "3 MAR 1901"},
		{"sometime in spring", "", "(sometime in spring)"},
		{"", "", ""},
		{"1850-13-01", "", "(1850-13-01)"},
	}
	for _, c := range cases {
		d := Parse(c.in)
		if got := d.SortKey(); got != c.sort {
			t.Errorf("Parse(%q).SortKey() = %q, want %q", c.in, got, c.sort)
		}
		if got := d.GEDCOM(); got != c.ged {
			t.Errorf("Parse(%q).GEDCOM() = %q, want %q", c.in, got, c.ged)
		}
	}
}

func TestSortOrder(t *testing.T) {
	order := []string{"BEF 1850", "1850", "1850-01", "1850-01-15", "AFT 1850", "1851"}
	for i := 1; i < len(order); i++ {
		a, b := Parse(order[i-1]).SortKey(), Parse(order[i]).SortKey()
		if !(a < b) {
			t.Errorf("expected %q (%s) < %q (%s)", order[i-1], a, order[i], b)
		}
	}
}

func TestFromGEDCOM(t *testing.T) {
	for in, want := range map[string]string{
		"12 MAR 1850": "1850-03-12", "MAR 1850": "1850-03", "1850": "1850",
		"ABT 1850": "ABT 1850", "(family bible)": "family bible",
	} {
		if got := FromGEDCOM(in); got != want {
			t.Errorf("FromGEDCOM(%q) = %q, want %q", in, got, want)
		}
	}
}
