package gendate

import (
	"testing"
	"time"
)

func TestNormalizeISO(t *testing.T) {
	ok := map[string]string{
		"1890-03-12": "1890-03-12", "1890-3-5": "1890-03-05", "1890-03": "1890-03", "1890-3": "1890-03", "1890": "1890",
		"2000-02-29": "2000-02-29", "0850-01-01": "0850-01-01",
	}
	for in, want := range ok {
		if got, err := NormalizeISO(in); err != nil || got != want {
			t.Errorf("NormalizeISO(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ABT 1850", "12 MAR 1850", "1890-13-01", "1900-02-29", "1890-04-31", "1890-00", "0000", "90-03-12", "1890/03/12", "1890-03-12T00:00", " 1890"} {
		if got, err := NormalizeISO(bad); err == nil {
			t.Errorf("NormalizeISO(%q) = %q, want error", bad, got)
		}
	}
	if !IsISO("1890-03-05") || IsISO("1890-3-5") || IsISO("ABT 1850") {
		t.Error("IsISO")
	}
}

func TestCompareISO(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1850", "1850-06-01", 0}, {"1849-12", "1850", -1}, {"1850-03-12", "1850-03-11", 1}, {"1850-03", "1850-03-31", 0},
	}
	for _, c := range cases {
		if got := CompareISO(c.a, c.b); got != c.want {
			t.Errorf("CompareISO(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAfterToday(t *testing.T) {
	now := time.Date(2026, 11, 27, 23, 0, 0, 0, time.UTC)
	for iso, want := range map[string]bool{
		"2026": false, "2026-11": false, "2026-11-28": false, // tomorrow in UTC is still "today" somewhere
		"2026-11-29": true, "2026-12": true, "2027": true, "1890-03-12": false,
	} {
		if got := AfterToday(iso, now); got != want {
			t.Errorf("AfterToday(%s) = %v, want %v", iso, got, want)
		}
	}
}
