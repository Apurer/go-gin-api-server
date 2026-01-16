package petstoreserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	pethttpmapper "github.com/Apurer/go-gin-api-server/internal/domains/pets/adapters/http/mapper"
	petsapp "github.com/Apurer/go-gin-api-server/internal/domains/pets/application"
	petstypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
	petsports "github.com/Apurer/go-gin-api-server/internal/domains/pets/ports"
	apierrors "github.com/Apurer/go-gin-api-server/internal/shared/errors"
)

// PetAPI wires HTTP transport with the pets bounded context service and workflows.
type PetAPI struct {
	service   petsports.Service
	workflows petsports.WorkflowOrchestrator
}

// NewPetAPI creates a PetAPI backed by the provided service.
func NewPetAPI(service petsports.Service, workflows petsports.WorkflowOrchestrator) PetAPI {
	return PetAPI{service: service, workflows: workflows}
}

// Post /v2/pet
// Add a new pet to the store
func (api *PetAPI) AddPet(c *gin.Context) {
	var payload PetCreate
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondProblem(c, apierrors.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	mutation := toMutationFromCreate(payload)
	input := petstypes.AddPetInput{PetMutationInput: pethttpmapper.ToMutationInput(mutation)}
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
	var payload PetUpdate
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondProblem(c, apierrors.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	mutation := toMutationFromUpdate(payload)
	input := petstypes.UpdatePetInput{PetMutationInput: pethttpmapper.ToMutationInput(mutation)}
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
	var payload GroomingOperation
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondProblem(c, apierrors.ErrBadRequest.WithDetail(err.Error()))
		return
	}
	input, err := pethttpmapper.ToGroomPetInput(id, toGroomOperation(payload))
	if err != nil {
		respondProblem(c, apierrors.ErrBadRequest.WithDetail(err.Error()))
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
		respondProblem(c, apierrors.ErrBadRequest.WithDetail(err.Error()))
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
		respondProblem(c, apierrors.ErrBadRequest.WithDetail(err.Error()))
		return 0, false
	}
	return id, true
}

func respondPetServiceError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if err == petsports.ErrNotFound {
		respondProblem(c, apierrors.ErrNotFound.WithDetail(err.Error()))
		return
	}
	if errors.Is(err, petsapp.ErrInvalidInput) {
		respondProblem(c, apierrors.ErrValidation.WithDetail(err.Error()))
		return
	}
	respondProblem(c, apierrors.ErrInternal.WithDetail(err.Error()))
}

func toMutationFromCreate(model PetCreate) pethttpmapper.MutationPet {
	mutation := pethttpmapper.MutationPet{ID: model.Id}
	name := model.Name
	mutation.Name = &name

	urls := append([]string{}, model.PhotoUrls...)
	mutation.PhotoURLs = &urls

	if !isEmptyCategory(model.Category) {
		mutation.Category = &pethttpmapper.Category{ID: model.Category.Id, Name: model.Category.Name}
	}
	if len(model.Tags) > 0 {
		tags := make([]pethttpmapper.Tag, 0, len(model.Tags))
		for _, tag := range model.Tags {
			tags = append(tags, pethttpmapper.Tag{ID: tag.Id, Name: tag.Name})
		}
		mutation.Tags = &tags
	}
	if model.Status != "" {
		status := model.Status
		mutation.Status = &status
	}
	if model.HairLengthCm != 0 {
		value := model.HairLengthCm
		mutation.HairLengthCm = &value
	}
	if ref := toExternalReference(model.ExternalReference); ref != nil {
		mutation.ExternalReference = ref
	}
	return mutation
}

func toMutationFromUpdate(model PetUpdate) pethttpmapper.MutationPet {
	mutation := pethttpmapper.MutationPet{ID: model.Id}
	if !isEmptyCategory(model.Category) {
		mutation.Category = &pethttpmapper.Category{ID: model.Category.Id, Name: model.Category.Name}
	}
	if model.Name != nil {
		name := *model.Name
		mutation.Name = &name
	}
	if model.PhotoUrls != nil {
		urls := append([]string{}, (*model.PhotoUrls)...)
		mutation.PhotoURLs = &urls
	}
	if model.Tags != nil {
		tags := make([]pethttpmapper.Tag, 0, len(*model.Tags))
		for _, tag := range *model.Tags {
			tags = append(tags, pethttpmapper.Tag{ID: tag.Id, Name: tag.Name})
		}
		mutation.Tags = &tags
	}
	if model.Status != nil {
		status := *model.Status
		mutation.Status = &status
	}
	if model.HairLengthCm != nil {
		value := *model.HairLengthCm
		mutation.HairLengthCm = &value
	}
	if ref := toExternalReference(model.ExternalReference); ref != nil {
		mutation.ExternalReference = ref
	}
	return mutation
}

func toGroomOperation(model GroomingOperation) pethttpmapper.GroomingOperation {
	initial := model.InitialHairLengthCm
	trim := model.TrimByCm
	return pethttpmapper.GroomingOperation{
		InitialHairLengthCm: &initial,
		TrimByCm:            &trim,
	}
}

func toExternalReference(ref ExternalPetReference) *pethttpmapper.ExternalReference {
	if ref.Provider == "" && ref.ExternalId == "" && len(ref.Attributes) == 0 {
		return nil
	}
	return &pethttpmapper.ExternalReference{
		Provider:   ref.Provider,
		ID:         ref.ExternalId,
		Attributes: pethttpmapper.CloneAttributes(ref.Attributes),
	}
}

func isEmptyCategory(cat Category) bool {
	return cat.Id == 0 && strings.TrimSpace(cat.Name) == ""
}
