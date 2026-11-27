package store

// Relationship types.
const (
	RelParent = "parent" // Person1 is a parent of Person2
	RelSpouse = "spouse" // Person1 and Person2 are partners (stored low id first)
)

// ParentSubtypes and SpouseSubtypes list allowed subtypes.
var (
	ParentSubtypes = []string{"biological", "adopted", "step", "foster", "guardian", "unknown"}
	SpouseSubtypes = []string{"married", "partner", "engaged", "divorced", "separated", "widowed", "unknown"}
)

// Relationship links two people.
type Relationship struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Person1ID int64  `json:"person1Id"`
	Person2ID int64  `json:"person2Id"`
	Subtype   string `json:"subtype"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Notes     string `json:"notes"`
	Attribution
}

// RelationshipInput holds editable relationship fields.
type RelationshipInput struct {
	Type      string `json:"type"`
	Person1ID int64  `json:"person1Id"`
	Person2ID int64  `json:"person2Id"`
	Subtype   string `json:"subtype"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
	Notes     string `json:"notes"`
}

// FindRelationship filters relationships. Empty filters match everything.
type FindRelationship struct {
	PersonID  *int64  // either side
	Type      string  // "" = any
	ChildIDs  []int64 // parent links whose child is in the set
	ParentIDs []int64 // parent links whose parent is in the set
}
