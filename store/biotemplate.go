package store

import (
	"strings"
	"unicode/utf8"
)

// EntityBioTemplate is the audit-log entity type for biography templates.
const EntityBioTemplate = "bio_template"

// BioTemplate is an admin-managed Markdown template for a person's About section.
type BioTemplate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ContentMD   string `json:"contentMd"`
	SortOrder   int    `json:"sortOrder"`
	Attribution
}

// BioTemplateInput holds the editable fields of a template.
type BioTemplateInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ContentMD   string `json:"contentMd"`
	SortOrder   int    `json:"sortOrder"`
}

// Normalize trims and validates the input.
func (in *BioTemplateInput) Normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	v := &validator{}
	v.check(in.Name != "", "name", "is required")
	v.check(utf8.RuneCountInString(in.Name) <= 80, "name", "must be at most 80 characters")
	v.check(utf8.RuneCountInString(in.Description) <= 200, "description", "must be at most 200 characters")
	v.check(strings.TrimSpace(in.ContentMD) != "", "contentMd", "the template needs some content")
	v.check(len(in.ContentMD) <= 50_000, "contentMd", "is too long")
	v.check(in.SortOrder >= -10000 && in.SortOrder <= 10000, "sortOrder", "must be between -10000 and 10000")
	return v.err()
}

// DefaultBioTemplate is seeded once into a new database.
var DefaultBioTemplate = BioTemplateInput{
	Name:        "Standard Biography",
	Description: "A gentle outline: where they lived, what they loved, and the stories worth keeping.",
	SortOrder:   0,
	ContentMD: `## Early Life
_Where and when were they born? Who raised them, and what was home like?_

## Places Lived
- _Town or city — years — what brought them there_

## Work & Daily Life
_What did they do for a living? How did they spend an ordinary day?_

## Favorite Things
- **Music:** 
- **Books or films:** 
- **Hobbies:** 

## Favorite Food
_A dish they loved to cook or eat — and any family recipe that came from them._

## Memorable Stories
_A story the family still tells. Who was there, and why does it stay with us?_
`,
}
