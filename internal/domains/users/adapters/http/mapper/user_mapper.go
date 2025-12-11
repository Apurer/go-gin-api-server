package mapper

import userdomain "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/users/domain"

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
func ToDomainUser(model User) (*userdomain.User, error) {
	user, err := userdomain.NewUser(model.ID, model.Username, model.Password)
	if err != nil {
		return nil, err
	}
	if err := user.UpdateProfile(model.FirstName, model.LastName, model.Email, model.Phone); err != nil {
		return nil, err
	}
	user.UpdateStatus(model.Status)
	return user, nil
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
func ToDomainUsers(users []User) ([]*userdomain.User, error) {
	result := make([]*userdomain.User, 0, len(users))
	for _, user := range users {
		u := user
		mapped, err := ToDomainUser(u)
		if err != nil {
			return nil, err
		}
		result = append(result, mapped)
	}
	return result, nil
}
