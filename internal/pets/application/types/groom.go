package types

// GroomPetInput carries the transient measurement data used to groom a pet.
type GroomPetInput struct {
	ID                  int64
	InitialHairLengthCm float64
	TrimByCm            float64
}
