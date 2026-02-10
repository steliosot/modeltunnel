package models

// Category represents a model use case category
type Category string

const (
	CategoryGeneral   Category = "general"
	CategoryCoding    Category = "coding"
	CategoryVision    Category = "vision"
	CategoryReasoning Category = "reasoning"
)

// ModelInfo contains metadata for a model
type ModelInfo struct {
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Size        string   `json:"size"`    // e.g., "4.1GB"
	Context     string   `json:"context"` // e.g., "32K"
	Description string   `json:"description"`
}

// RecommendedModels is a curated list of models with metadata
var RecommendedModels = []ModelInfo{
	// General Purpose
	{
		Name:        "llama3.2:3b",
		Category:    CategoryGeneral,
		Size:        "2.3GB",
		Context:     "8K",
		Description: "Well-rounded, balanced performance",
	},
	{
		Name:        "mistral:7b",
		Category:    CategoryGeneral,
		Size:        "4.1GB",
		Context:     "32K",
		Description: "Fast, intelligent, versatile",
	},
	{
		Name:        "phi4",
		Category:    CategoryGeneral,
		Size:        "2.7GB",
		Context:     "32K",
		Description: "Small but powerful, reasoning",
	},

	// Coding
	{
		Name:        "codellama:7b",
		Category:    CategoryCoding,
		Size:        "4.2GB",
		Context:     "32K",
		Description: "Code generation and completion",
	},
	{
		Name:        "deepseek-coder:6.7b",
		Category:    CategoryCoding,
		Size:        "4.5GB",
		Context:     "32K",
		Description: "Competitive programming, coding",
	},

	// Vision
	{
		Name:        "llava:7b",
		Category:    CategoryVision,
		Size:        "4.5GB",
		Context:     "32K",
		Description: "Image understanding, vision-language",
	},

	// Reasoning
	{
		Name:        "deepseek-r1:8b",
		Category:    CategoryReasoning,
		Size:        "4.8GB",
		Context:     "32K",
		Description: "Math, logic, reasoning tasks",
	},
}

// GetRecommendedByCategory returns models filtered by category
func GetRecommendedByCategory(category Category) []ModelInfo {
	var result []ModelInfo
	for _, m := range RecommendedModels {
		if m.Category == category {
			result = append(result, m)
		}
	}
	return result
}

// GetRecommendedModel returns metadata for a specific model name
func GetRecommendedModel(name string) *ModelInfo {
	for _, m := range RecommendedModels {
		if m.Name == name {
			return &m
		}
	}
	return nil
}
