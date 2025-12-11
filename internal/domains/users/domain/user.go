package domain

import (
	"errors"
	"strings"
)

var (
	ErrEmptyUsername  = errors.New("username is required")
	ErrEmptyPassword  = errors.New("password is required")
	ErrInvalidEmail   = errors.New("email must contain '@'")
	ErrWeakPassword   = errors.New("password must be at least 4 characters")
)

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

// NewUser builds a user ensuring required invariants.
func NewUser(id int64, username, password string) (*User, error) {
	user := &User{ID: id}
	if err := user.SetUsername(username); err != nil {
		return nil, err
	}
	if err := user.SetPassword(password); err != nil {
		return nil, err
	}
	return user, nil
}

// SetUsername trims and validates the username.
func (u *User) SetUsername(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ErrEmptyUsername
	}
	u.Username = username
	return nil
}

// SetPassword validates basic password strength.
func (u *User) SetPassword(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return ErrEmptyPassword
	}
	if len(password) < 4 {
		return ErrWeakPassword
	}
	u.Password = password
	return nil
}

// UpdateProfile applies optional profile fields and validates email if present.
func (u *User) UpdateProfile(firstName, lastName, email, phone string) error {
	u.FirstName = strings.TrimSpace(firstName)
	u.LastName = strings.TrimSpace(lastName)
	email = strings.TrimSpace(email)
	if email != "" && !strings.Contains(email, "@") {
		return ErrInvalidEmail
	}
	u.Email = email
	u.Phone = strings.TrimSpace(phone)
	return nil
}

// UpdateStatus sets the user status flag.
func (u *User) UpdateStatus(status int32) {
	u.Status = status
}

// CheckPassword compares the stored password with the supplied credentials.
func (u *User) CheckPassword(password string) bool {
	return strings.TrimSpace(password) != "" && u.Password == strings.TrimSpace(password)
}

// Validate re-applies core invariants for persistence.
func (u *User) Validate() error {
	if err := u.SetUsername(u.Username); err != nil {
		return err
	}
	if err := u.SetPassword(u.Password); err != nil {
		return err
	}
	if err := u.UpdateProfile(u.FirstName, u.LastName, u.Email, u.Phone); err != nil {
		return err
	}
	return nil
}
