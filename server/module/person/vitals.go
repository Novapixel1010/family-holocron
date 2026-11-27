package person

import (
	"time"

	"github.com/novapixel1010/family-holocron/internal/gendate"
	"github.com/novapixel1010/family-holocron/store"
)

// checkVitalDates enforces the YYYY-MM-DD standard for birth and death dates on
// API writes, normalizing padding in place ("1890-3-5" → "1890-03-05").
//
// YYYY-MM and YYYY are accepted when the day or month is unknown. A value that
// predates the standard (e.g. "ABT 1850" from a GEDCOM import) is accepted only
// if it is unchanged from the stored record, so editing other fields never fails
// because of an old date. Death may not precede birth, and neither may be in the future.
func checkVitalDates(in *store.PersonInput, before *store.Person, now time.Time) error {
	fields := map[string]string{}
	check := func(field string, value *string, stored string) (iso string) {
		v := *value
		if v == "" {
			return ""
		}
		n, err := gendate.NormalizeISO(v)
		if err != nil {
			if before != nil && v == stored {
				return "" // unchanged legacy value: keep as is, skip comparisons
			}
			fields[field] = err.Error()
			return ""
		}
		if gendate.AfterToday(n, now) {
			fields[field] = "can't be in the future"
			return ""
		}
		*value = n
		return n
	}
	var storedBirth, storedDeath string
	if before != nil {
		storedBirth, storedDeath = before.BirthDate, before.DeathDate
	}
	birth := check("birthDate", &in.BirthDate, storedBirth)
	death := check("deathDate", &in.DeathDate, storedDeath)
	if birth != "" && death != "" && gendate.CompareISO(death, birth) < 0 {
		fields["deathDate"] = "can't be before the birth date (" + birth + ")"
	}
	if len(fields) > 0 {
		return &store.ValidationError{Message: "Please correct the highlighted dates.", Fields: fields}
	}
	return nil
}
