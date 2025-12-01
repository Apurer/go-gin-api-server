package types

// UploadImageInput represents the command to attach media to a pet.
type UploadImageInput struct {
	ID       int64
	Filename string
	Metadata string
}
