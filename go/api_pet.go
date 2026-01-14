package petstoreserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	pethttpmapper "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/adapters/http/mapper"
	petsapp "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application"
	petstypes "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/application/types"
	petsports "github.com/GIT_USER_ID/GIT_REPO_ID/internal/domains/pets/ports"
)

// PetAPI wires HTTP transport with the pets bounded context service and workflows.
type PetAPI struct {
	service   petsapp.Port
	workflows petsports.WorkflowOrchestrator
}

// NewPetAPI creates a PetAPI backed by the provided service.
func NewPetAPI(service petsapp.Port, workflows petsports.WorkflowOrchestrator) PetAPI {
	return PetAPI{service: service, workflows: workflows}
}

// Post /v2/pet
// Add a new pet to the store
func (api *PetAPI) AddPet(c *gin.Context) {
	var payload pethttpmapper.MutationPet
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	input := petstypes.AddPetInput{PetMutationInput: pethttpmapper.ToMutationInput(payload)}
	saved, err := api.createPet(c.Request.Context(), input)
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjection(saved))
}

func (api *PetAPI) createPet(ctx context.Context, input petstypes.AddPetInput) (*petstypes.PetProjection, error) {
	if api.workflows != nil {
		return api.workflows.CreatePet(ctx, input)
	}
	return api.service.AddPet(ctx, input)
}

// Delete /v2/pet/:petId
// Deletes a pet
func (api *PetAPI) DeletePet(c *gin.Context) {
	id, ok := parseIDParam(c, "petId")
	if !ok {
		return
	}
	if err := api.service.Delete(c.Request.Context(), petstypes.PetIdentifier{ID: id}); err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// Get /v2/pet/findByStatus
// Finds Pets by status
func (api *PetAPI) FindPetsByStatus(c *gin.Context) {
	statuses := c.QueryArray("status")
	result, err := api.service.FindByStatus(c.Request.Context(), petstypes.FindPetsByStatusInput{Statuses: statuses})
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjectionList(result))
}

// Get /v2/pet/findByTags
// Finds Pets by tags
// Deprecated
func (api *PetAPI) FindPetsByTags(c *gin.Context) {
	tags := c.QueryArray("tags")
	result, err := api.service.FindByTags(c.Request.Context(), petstypes.FindPetsByTagsInput{Tags: tags})
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjectionList(result))
}

// Get /v2/pet/:petId
// Find pet by ID
func (api *PetAPI) GetPetById(c *gin.Context) {
	id, ok := parseIDParam(c, "petId")
	if !ok {
		return
	}
	pet, err := api.service.GetByID(c.Request.Context(), petstypes.PetIdentifier{ID: id})
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjection(pet))
}

// Put /v2/pet
// Update an existing pet
func (api *PetAPI) UpdatePet(c *gin.Context) {
	var payload pethttpmapper.MutationPet
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	input := petstypes.UpdatePetInput{PetMutationInput: pethttpmapper.ToMutationInput(payload)}
	updated, err := api.service.UpdatePet(c.Request.Context(), input)
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjection(updated))
}

// Post /v2/pet/:petId
// Updates a pet in the store with form data
func (api *PetAPI) UpdatePetWithForm(c *gin.Context) {
	id, ok := parseIDParam(c, "petId")
	if !ok {
		return
	}
	name := c.PostForm("name")
	status := c.PostForm("status")
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	var statusPtr *string
	if status != "" {
		statusPtr = &status
	}
	input := petstypes.UpdatePetWithFormInput{ID: id, Name: namePtr, Status: statusPtr}
	updated, err := api.service.UpdatePetWithForm(c.Request.Context(), input)
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjection(updated))
}

// Post /v2/pet/:petId/groom
// Applies a grooming operation using transient hair measurements
func (api *PetAPI) GroomPet(c *gin.Context) {
	id, ok := parseIDParam(c, "petId")
	if !ok {
		return
	}
	var payload pethttpmapper.GroomingOperation
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	input, err := pethttpmapper.ToGroomPetInput(id, payload)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	updated, err := api.service.GroomPet(c.Request.Context(), input)
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, pethttpmapper.FromProjection(updated))
}

// Post /v2/pet/:petId/uploadImage
// uploads an image
func (api *PetAPI) UploadFile(c *gin.Context) {
	id, ok := parseIDParam(c, "petId")
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	metadata := c.PostForm("additionalMetadata")
	input := petstypes.UploadImageInput{ID: id, Filename: file.Filename, Metadata: metadata}
	result, err := api.service.UploadImage(c.Request.Context(), input)
	if err != nil {
		respondPetServiceError(c, err)
		return
	}
	response := ApiResponse{Code: result.Code, Type: result.Type, Message: result.Message}
	c.JSON(http.StatusOK, response)
}

func parseIDParam(c *gin.Context, name string) (int64, bool) {
	value := c.Param(name)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return 0, false
	}
	return id, true
}

func respondError(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"error": err.Error()})
}

func respondPetServiceError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if err == petsports.ErrNotFound {
		respondError(c, http.StatusNotFound, err)
		return
	}
	if errors.Is(err, petsapp.ErrInvalidInput) {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	respondError(c, http.StatusInternalServerError, err)
}
