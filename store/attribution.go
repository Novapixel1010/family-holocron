package store

import "time"

// Actor identifies who performs a mutation. It is recorded in attribution columns
// and in the audit log.
type Actor struct {
	UserID   int64
	Username string
}

// SystemActor is used for changes not performed by a signed-in user.
var SystemActor = Actor{Username: "system"}

// UserRef is the compact user reference exposed in attribution.
type UserRef struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Attribution records who created and last updated a record, and when.
type Attribution struct {
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy *UserRef  `json:"createdBy"`
	UpdatedBy *UserRef  `json:"updatedBy"`
}
