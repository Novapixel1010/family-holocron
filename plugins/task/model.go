package task

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/novapixel1010/family-holocron/store"
)

// Statuses and priorities.
const (
	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusDone       = "done"
)

var (
	Statuses   = []string{StatusOpen, StatusInProgress, StatusBlocked, StatusDone}
	Priorities = []string{"low", "medium", "high"}
)

// EntityTask is the audit-log entity type for tasks.
const EntityTask = "task"

// PersonRef is a linked person.
type PersonRef struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	BirthDate string `json:"birthDate"`
	DeathDate string `json:"deathDate"`
}

// MediaRef is a linked media item.
type MediaRef struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	OriginalName string `json:"originalName"`
	MimeType     string `json:"mimeType"`
}

// Task is a research task.
type Task struct {
	ID            int64          `json:"id"`
	Title         string         `json:"title"`
	DescriptionMD string         `json:"descriptionMd"`
	Status        string         `json:"status"`
	Priority      string         `json:"priority"`
	Tags          []string       `json:"tags"`
	ClaimedBy     *store.UserRef `json:"claimedBy"`
	ClaimedAt     *time.Time     `json:"claimedAt"`
	People        []PersonRef    `json:"people"`
	Media         []MediaRef     `json:"media"`
	CommentCount  int            `json:"commentCount"`
	store.Attribution
}

// auditView is the subset of a task compared in the change log.
func (t *Task) auditView() map[string]any {
	if t == nil {
		return nil
	}
	people := make([]string, 0, len(t.People))
	for _, p := range t.People {
		people = append(people, p.Name)
	}
	media := make([]string, 0, len(t.Media))
	for _, m := range t.Media {
		media = append(media, firstNonEmpty(m.Title, m.OriginalName))
	}
	claimed := ""
	if t.ClaimedBy != nil {
		claimed = firstNonEmpty(t.ClaimedBy.DisplayName, t.ClaimedBy.Username)
	}
	return map[string]any{
		"title": t.Title, "description": t.DescriptionMD, "status": t.Status, "priority": t.Priority,
		"tags": t.Tags, "people": people, "media": media, "claimedBy": claimed,
	}
}

func (t *Task) personIDs() []int64 {
	if t == nil {
		return nil
	}
	ids := make([]int64, 0, len(t.People))
	for _, p := range t.People {
		ids = append(ids, p.ID)
	}
	return ids
}

// Input holds the editable fields of a task.
type Input struct {
	Title         string   `json:"title"`
	DescriptionMD string   `json:"descriptionMd"`
	Status        string   `json:"status"`
	Priority      string   `json:"priority"`
	Tags          []string `json:"tags"`
	PersonIDs     []int64  `json:"personIds"`
	MediaIDs      []int64  `json:"mediaIds"`
}

// Normalize trims and validates the input.
func (in *Input) Normalize() error {
	fields := map[string]string{}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		fields["title"] = "is required"
	} else if utf8.RuneCountInString(in.Title) > 200 {
		fields["title"] = "must be at most 200 characters"
	}
	if len(in.DescriptionMD) > 200_000 {
		fields["descriptionMd"] = "is too long"
	}
	if in.Status == "" {
		in.Status = StatusOpen
	}
	if !slices.Contains(Statuses, in.Status) {
		fields["status"] = "must be open, in_progress, blocked or done"
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if !slices.Contains(Priorities, in.Priority) {
		fields["priority"] = "must be low, medium or high"
	}
	var tags []string
	seen := map[string]bool{}
	for _, t := range in.Tags {
		t = strings.Join(strings.Fields(strings.ReplaceAll(t, ",", " ")), " ")
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		if utf8.RuneCountInString(t) > 40 {
			fields["tags"] = "each tag must be at most 40 characters"
			break
		}
		seen[strings.ToLower(t)] = true
		tags = append(tags, t)
	}
	if len(tags) > 20 {
		fields["tags"] = "at most 20 tags"
	}
	in.Tags = tags
	in.PersonIDs = uniq(in.PersonIDs)
	in.MediaIDs = uniq(in.MediaIDs)
	if len(fields) > 0 {
		return &store.ValidationError{Message: "Please correct the highlighted fields.", Fields: fields}
	}
	return nil
}

func uniq(ids []int64) []int64 {
	out := []int64{}
	for _, id := range ids {
		if id > 0 && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// Filter selects tasks.
type Filter struct {
	Status    []string
	Priority  string
	PersonID  *int64
	Tag       string
	ClaimedBy *int64
	Query     string
}
