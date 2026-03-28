package llm

import (
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
)

// ConfigPromptsAdapter adapts config.PromptsConfig to Router.PromptsConfig interface.
type ConfigPromptsAdapter struct {
	prompts config.PromptsConfig
}

// NewConfigPromptsAdapter creates a new adapter.
func NewConfigPromptsAdapter(prompts config.PromptsConfig) *ConfigPromptsAdapter {
	return &ConfigPromptsAdapter{prompts: prompts}
}

// GetSuggestTags returns the tag suggestion prompt.
func (a *ConfigPromptsAdapter) GetSuggestTags() string {
	if a.prompts.SuggestTags != "" {
		return a.prompts.SuggestTags
	}
	return config.DefaultPromptsConfig().SuggestTags
}

// GetSuggestProject returns the project suggestion prompt.
func (a *ConfigPromptsAdapter) GetSuggestProject() string {
	if a.prompts.SuggestProject != "" {
		return a.prompts.SuggestProject
	}
	return config.DefaultPromptsConfig().SuggestProject
}

// GetGenerateSummary returns the summary generation prompt.
func (a *ConfigPromptsAdapter) GetGenerateSummary() string {
	if a.prompts.GenerateSummary != "" {
		return a.prompts.GenerateSummary
	}
	return config.DefaultPromptsConfig().GenerateSummary
}

// GetExtractKeyTerms returns the key terms extraction prompt.
func (a *ConfigPromptsAdapter) GetExtractKeyTerms() string {
	if a.prompts.ExtractKeyTerms != "" {
		return a.prompts.ExtractKeyTerms
	}
	return config.DefaultPromptsConfig().ExtractKeyTerms
}

// GetRAGAnswer returns the RAG answer prompt.
func (a *ConfigPromptsAdapter) GetRAGAnswer() string {
	if a.prompts.RAGAnswer != "" {
		return a.prompts.RAGAnswer
	}
	return config.DefaultPromptsConfig().RAGAnswer
}

// GetClassifyCategory returns the category classification prompt.
func (a *ConfigPromptsAdapter) GetClassifyCategory() string {
	if a.prompts.ClassifyCategory != "" {
		return a.prompts.ClassifyCategory
	}
	return config.DefaultPromptsConfig().ClassifyCategory
}

