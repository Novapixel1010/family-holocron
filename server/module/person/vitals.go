package person

import (
	"strings"
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
	var storedBirth, storedDeath string
	if before != nil {
		storedBirth, storedDeath = before.BirthDate, before.DeathDate
	}
	return checkRange(&in.BirthDate, storedBirth, "birthDate", &in.DeathDate, storedDeath, "deathDate",
		"can't be before the birth date", false, now)
}

// checkRelationshipDates applies the same standard to a relationship's start/end dates.
func checkRelationshipDates(in *store.RelationshipInput, before *store.Relationship, now time.Time) error {
	var storedStart, storedEnd string
	if before != nil {
		storedStart, storedEnd = before.StartDate, before.EndDate
	}
	return checkRange(&in.StartDate, storedStart, "startDate", &in.EndDate, storedEnd, "endDate",
		"can't be before the start date", false, now)
}

// checkRange validates a start/end pair of date fields in place.
func checkRange(start *string, storedStart, startField string, end *string, storedEnd, endField, beforeMsg string, allowFuture bool, now time.Time) error {
	fields := map[string]string{}
	*start, *end = strings.TrimSpace(*start), strings.TrimSpace(*end)
	s, sISO, msg := gendate.CheckField(*start, storedStart, allowFuture, now)
	if msg != "" {
		fields[startField] = msg
	}
	e, eISO, msg := gendate.CheckField(*end, storedEnd, allowFuture, now)
	if msg != "" {
		fields[endField] = msg
	}
	if sISO && eISO && fields[endField] == "" && gendate.CompareISO(e, s) < 0 {
		fields[endField] = beforeMsg + " (" + s + ")"
	}
	if len(fields) > 0 {
		return &store.ValidationError{Message: "Please correct the highlighted dates.", Fields: fields}
	}
	*start, *end = s, e
	return nil
}
