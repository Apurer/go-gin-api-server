package types

// CategoryInput describes the category payload supplied to pet use cases.
type CategoryInput struct {
	ID   int64
	Name string
}

// TagInput carries tag metadata for pet commands.
type TagInput struct {
	ID   int64
	Name string
}

// ExternalReferenceInput links a local pet to an upstream provider record.
type ExternalReferenceInput struct {
	Provider   string
	ID         string
	Attributes map[string]string
}

// PetMutationInput represents the full set of fields required to create or replace a pet aggregate.
type PetMutationInput struct {
	ID                int64
	Name              *string
	PhotoURLs         *[]string
	Category          *CategoryInput
	Tags              *[]TagInput
	Status            *string
	HairLengthCm      *float64
	ExternalReference *ExternalReferenceInput
}

// AddPetInput captures the request to add a new pet into the catalog.
type AddPetInput struct {
	PetMutationInput
}

// UpdatePetInput replaces an existing pet with new state.
type UpdatePetInput struct {
	PetMutationInput
}

// UpdatePetWithFormInput models the simplified form-based update flow.
type UpdatePetWithFormInput struct {
	ID     int64
	Name   *string
	Status *string
}
