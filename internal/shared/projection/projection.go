package projection

import "time"

// Metadata captures persistence timestamps shared by projections.
type Metadata struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Projection represents an aggregate view plus persistence metadata.
type Projection[T any] struct {
	Entity   T
	Metadata Metadata
}
