package http

import userdomain "github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/domain"

// User represents the transport-level user payload.
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

// ToDomainUser converts a transport user to its domain counterpart.
func ToDomainUser(model User) *userdomain.User {
	return &userdomain.User{
		ID:        model.ID,
		Username:  model.Username,
		FirstName: model.FirstName,
		LastName:  model.LastName,
		Email:     model.Email,
		Password:  model.Password,
		Phone:     model.Phone,
		Status:    model.Status,
	}
}

// FromDomainUser converts a domain user into a transport representation.
func FromDomainUser(user *userdomain.User) User {
	if user == nil {
		return User{}
	}
	return User{
		ID:        user.ID,
		Username:  user.Username,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
		Password:  user.Password,
		Phone:     user.Phone,
		Status:    user.Status,
	}
}

// FromDomainUsers converts a slice of domain users to transport representation.
func FromDomainUsers(users []*userdomain.User) []User {
	result := make([]User, 0, len(users))
	for _, user := range users {
		result = append(result, FromDomainUser(user))
	}
	return result
}

// ToDomainUsers converts transport users into the domain representation.
func ToDomainUsers(users []User) []*userdomain.User {
	result := make([]*userdomain.User, 0, len(users))
	for _, user := range users {
		u := user
		result = append(result, ToDomainUser(u))
	}
	return result
}
