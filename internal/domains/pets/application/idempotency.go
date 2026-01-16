package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	pettypes "github.com/Apurer/go-gin-api-server/internal/domains/pets/application/types"
)

type normalizedAddPetInput struct {
	ID                int64                           `json:"id"`
	Name              *string                         `json:"name"`
	PhotoURLs         *[]string                       `json:"photoUrls"`
	Category          *normalizedCategory             `json:"category"`
	Tags              *[]normalizedTag                `json:"tags"`
	Status            *string                         `json:"status"`
	HairLengthCm      *float64                        `json:"hairLengthCm"`
	ExternalReference *normalizedExternalReference    `json:"externalReference"`
}

type normalizedCategory struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type normalizedTag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type normalizedExternalReference struct {
	Provider   string            `json:"provider"`
	ID         string            `json:"id"`
	Attributes []normalizedAttrKV `json:"attributes,omitempty"`
}

type normalizedAttrKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// FingerprintAddPet builds a deterministic hash of the add-pet request payload (excluding the idempotency key).
func FingerprintAddPet(input pettypes.AddPetInput) (string, error) {
	normalized := normalizeAddPetInput(input)
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func normalizeAddPetInput(input pettypes.AddPetInput) normalizedAddPetInput {
	normalized := normalizedAddPetInput{
		ID:           input.ID,
		Name:         input.Name,
		PhotoURLs:    input.PhotoURLs,
		Category:     normalizeCategory(input.Category),
		Tags:         normalizeTags(input.Tags),
		Status:       input.Status,
		HairLengthCm: input.HairLengthCm,
	}
	if input.ExternalReference != nil {
		normalized.ExternalReference = normalizeExternalReference(input.ExternalReference)
	}
	return normalized
}

func normalizeCategory(cat *pettypes.CategoryInput) *normalizedCategory {
	if cat == nil {
		return nil
	}
	return &normalizedCategory{ID: cat.ID, Name: cat.Name}
}

func normalizeTags(tags *[]pettypes.TagInput) *[]normalizedTag {
	if tags == nil {
		return nil
	}
	normalized := make([]normalizedTag, 0, len(*tags))
	for _, t := range *tags {
		normalized = append(normalized, normalizedTag{ID: t.ID, Name: t.Name})
	}
	return &normalized
}

func normalizeExternalReference(ref *pettypes.ExternalReferenceInput) *normalizedExternalReference {
	if ref == nil {
		return nil
	}
	attrs := make([]normalizedAttrKV, 0, len(ref.Attributes))
	for k, v := range ref.Attributes {
		attrs = append(attrs, normalizedAttrKV{Key: k, Value: v})
	}
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Key < attrs[j].Key })
	return &normalizedExternalReference{
		Provider:   ref.Provider,
		ID:         ref.ID,
		Attributes: attrs,
	}
}
