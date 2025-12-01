package domain

// User represents a Petstore user entity.
type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
	Email     string
	Password  string
	Phone     string
	Status    int32
}
