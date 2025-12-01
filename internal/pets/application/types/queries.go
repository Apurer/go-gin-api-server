package types

// FindPetsByStatusInput filters pets by store status.
type FindPetsByStatusInput struct {
	Statuses []string
}

// FindPetsByTagsInput filters pets by tag names.
type FindPetsByTagsInput struct {
	Tags []string
}

// PetIdentifier references a pet by its aggregate ID.
type PetIdentifier struct {
	ID int64
}
