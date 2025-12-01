package petstoreserver

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	userhttpmapper "github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/adapters/http/mapper"
	userapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/application"
	userports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/users/ports"
)

// UserAPI implements the user OpenAPI section.
type UserAPI struct {
	service *userapp.Service
}

// NewUserAPI wires dependencies.
func NewUserAPI(service *userapp.Service) UserAPI {
	return UserAPI{service: service}
}

func toTransportUser(model User) userhttpmapper.User {
	return userhttpmapper.User{
		ID:        model.Id,
		Username:  model.Username,
		FirstName: model.FirstName,
		LastName:  model.LastName,
		Email:     model.Email,
		Password:  model.Password,
		Phone:     model.Phone,
		Status:    model.UserStatus,
	}
}

func toTransportUserList(list []User) []userhttpmapper.User {
	result := make([]userhttpmapper.User, 0, len(list))
	for _, item := range list {
		result = append(result, toTransportUser(item))
	}
	return result
}

func fromTransportUser(user userhttpmapper.User) User {
	return User{
		Id:         user.ID,
		Username:   user.Username,
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		Email:      user.Email,
		Password:   user.Password,
		Phone:      user.Phone,
		UserStatus: user.Status,
	}
}

func fromTransportUsers(users []userhttpmapper.User) []User {
	result := make([]User, 0, len(users))
	for _, user := range users {
		result = append(result, fromTransportUser(user))
	}
	return result
}

// Post /v2/user
// Create user
func (api *UserAPI) CreateUser(c *gin.Context) {
	var payload User
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	user := userhttpmapper.ToDomainUser(toTransportUser(payload))
	saved, err := api.service.CreateUser(c.Request.Context(), user)
	if err != nil {
		respondUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, fromTransportUser(userhttpmapper.FromDomainUser(saved)))
}

// Post /v2/user/createWithArray
// Creates list of users with given input array
func (api *UserAPI) CreateUsersWithArrayInput(c *gin.Context) {
	var payload []User
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	users := userhttpmapper.ToDomainUsers(toTransportUserList(payload))
	created, err := api.service.CreateUsers(c.Request.Context(), users)
	if err != nil {
		respondUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, fromTransportUsers(userhttpmapper.FromDomainUsers(created)))
}

// Post /v2/user/createWithList
// Creates list of users with given input array
func (api *UserAPI) CreateUsersWithListInput(c *gin.Context) {
	var payload []User
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	users := userhttpmapper.ToDomainUsers(toTransportUserList(payload))
	created, err := api.service.CreateUsers(c.Request.Context(), users)
	if err != nil {
		respondUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, fromTransportUsers(userhttpmapper.FromDomainUsers(created)))
}

// Delete /v2/user/:username
// Delete user
func (api *UserAPI) DeleteUser(c *gin.Context) {
	username := c.Param("username")
	if strings.TrimSpace(username) == "" {
		respondError(c, http.StatusBadRequest, errors.New("username is required"))
		return
	}
	if err := api.service.Delete(c.Request.Context(), username); err != nil {
		respondUserError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// Get /v2/user/:username
// Get user by user name
func (api *UserAPI) GetUserByName(c *gin.Context) {
	username := c.Param("username")
	user, err := api.service.GetByUsername(c.Request.Context(), username)
	if err != nil {
		respondUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, fromTransportUser(userhttpmapper.FromDomainUser(user)))
}

// Get /v2/user/login
// Logs user into the system
func (api *UserAPI) LoginUser(c *gin.Context) {
	username := c.Query("username")
	password := c.Query("password")
	token, err := api.service.Login(c.Request.Context(), username, password)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.Header("Set-Cookie", "api_key="+token)
	c.JSON(http.StatusOK, gin.H{"message": "login successful", "token": token})
}

// Get /v2/user/logout
// Logs out current logged in user session
func (api *UserAPI) LogoutUser(c *gin.Context) {
	username := c.Query("username")
	if username != "" {
		api.service.Logout(c.Request.Context(), username)
	}
	c.JSON(http.StatusOK, gin.H{"message": "logout successful"})
}

// Put /v2/user/:username
// Updated user
func (api *UserAPI) UpdateUser(c *gin.Context) {
	username := c.Param("username")
	var payload User
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	user := userhttpmapper.ToDomainUser(toTransportUser(payload))
	updated, err := api.service.Update(c.Request.Context(), username, user)
	if err != nil {
		respondUserError(c, err)
		return
	}
	c.JSON(http.StatusOK, fromTransportUser(userhttpmapper.FromDomainUser(updated)))
}

func respondUserError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if err == userports.ErrNotFound {
		respondError(c, http.StatusNotFound, err)
		return
	}
	respondError(c, http.StatusInternalServerError, err)
}
