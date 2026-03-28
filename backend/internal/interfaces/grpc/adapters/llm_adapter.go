package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LLMServiceAdapter implements cortexv1.LLMServiceServer.
type LLMServiceAdapter struct {
	cortexv1.UnimplementedLLMServiceServer
	handler *handlers.LLMHandler
}

// NewLLMServiceAdapter creates a new LLM service adapter.
func NewLLMServiceAdapter(handler *handlers.LLMHandler) *LLMServiceAdapter {
	return &LLMServiceAdapter{handler: handler}
}

func (a *LLMServiceAdapter) ListProviders(ctx context.Context, req *cortexv1.ListProvidersRequest) (*cortexv1.ProviderList, error) {
	providers := a.handler.ListProviders()
	resp := &cortexv1.ProviderList{}
	for _, provider := range providers {
		status, err := a.handler.GetProviderStatus(ctx, provider.ID)
		available := false
		if err == nil && status != nil {
			available = status.Available
		}
		resp.Providers = append(resp.Providers, &cortexv1.LLMProvider{
			Id:        provider.ID,
			Name:      provider.Name,
			Type:      provider.Type,
			Endpoint:  "",
			Available: available,
		})
	}
	return resp, nil
}

func (a *LLMServiceAdapter) GetProviderStatus(ctx context.Context, req *cortexv1.GetProviderStatusRequest) (*cortexv1.ProviderStatus, error) {
	if req == nil || req.ProviderId == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_id is required")
	}
	statusInfo, err := a.handler.GetProviderStatus(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	resp := &cortexv1.ProviderStatus{
		Provider: &cortexv1.LLMProvider{
			Id:        statusInfo.Info.ID,
			Name:      statusInfo.Info.Name,
			Type:      statusInfo.Info.Type,
			Available: statusInfo.Available,
		},
		Connected:       statusInfo.Available,
		AvailableModels: modelNames(statusInfo.Models),
		ActiveModel:     "",
	}
	if statusInfo.Error != nil {
		resp.Error = statusInfo.Error
	}
	return resp, nil
}

func modelNames(models []llm.ModelInfo) []string {
	if len(models) == 0 {
		return nil
	}
	names := make([]string, 0, len(models))
	for _, model := range models {
		if model.Name == "" {
			continue
		}
		names = append(names, model.Name)
	}
	return names
}

func (a *LLMServiceAdapter) SetActiveProvider(ctx context.Context, req *cortexv1.SetActiveProviderRequest) (*cortexv1.ProviderStatus, error) {
	if req == nil || req.ProviderId == "" {
		return nil, status.Error(codes.InvalidArgument, "provider_id is required")
	}
	if err := a.handler.SetActiveProvider(req.ProviderId, req.GetModel()); err != nil {
		return nil, err
	}
	return a.GetProviderStatus(ctx, &cortexv1.GetProviderStatusRequest{ProviderId: req.ProviderId})
}

func (a *LLMServiceAdapter) ListModels(ctx context.Context, req *cortexv1.ListModelsRequest) (*cortexv1.ModelList, error) {
	providerID := ""
	if req != nil {
		providerID = req.GetProviderId()
	}
	models, err := a.handler.ListModels(ctx, providerID)
	if err != nil {
		return nil, err
	}
	resp := &cortexv1.ModelList{}
	for _, model := range models {
		resp.Models = append(resp.Models, &cortexv1.ModelInfo{
			Name:           model.Name,
			Provider:       providerID,
			ContextLength:  model.ContextLength,
			ParameterCount: 0,
			Capabilities:   model.Capabilities,
		})
	}
	return resp, nil
}

func (a *LLMServiceAdapter) GetModelInfo(ctx context.Context, req *cortexv1.GetModelInfoRequest) (*cortexv1.ModelInfo, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	models, err := a.handler.ListModels(ctx, req.ProviderId)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if model.Name == req.ModelName {
			return &cortexv1.ModelInfo{
				Name:           model.Name,
				Provider:       req.ProviderId,
				ContextLength:  model.ContextLength,
				ParameterCount: 0,
				Capabilities:   model.Capabilities,
			}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "model not found")
}

func (a *LLMServiceAdapter) SuggestTags(ctx context.Context, req *cortexv1.SuggestTagsRequest) (*cortexv1.TagSuggestions, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	content := req.GetContent()
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	tags, err := a.handler.SuggestTags(ctx, content, int(req.MaxTags))
	if err != nil {
		return nil, err
	}
	resp := &cortexv1.TagSuggestions{}
	for _, tag := range tags {
		resp.Suggestions = append(resp.Suggestions, &cortexv1.TagSuggestion{
			Tag: tag,
		})
	}
	return resp, nil
}

func (a *LLMServiceAdapter) SuggestProject(ctx context.Context, req *cortexv1.SuggestProjectRequest) (*cortexv1.ProjectSuggestion, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	content := req.GetContent()
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	project, err := a.handler.SuggestProject(ctx, content, req.ExistingProjects)
	if err != nil {
		return nil, err
	}
	if project == "" {
		return &cortexv1.ProjectSuggestion{}, nil
	}
	return &cortexv1.ProjectSuggestion{Project: &project}, nil
}

func (a *LLMServiceAdapter) GenerateSummary(ctx context.Context, req *cortexv1.GenerateSummaryRequest) (*cortexv1.SummaryResult, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	content := req.GetContent()
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	summary, err := a.handler.GenerateSummary(ctx, content, int(req.MaxLength))
	if err != nil {
		return nil, err
	}
	return &cortexv1.SummaryResult{
		Summary:     summary,
		ContentHash: hashContent(content),
	}, nil
}

func (a *LLMServiceAdapter) ClassifyCategory(ctx context.Context, req *cortexv1.ClassifyCategoryRequest) (*cortexv1.CategoryResult, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	content := req.GetContent()
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	category, err := a.handler.ClassifyCategory(ctx, content, req.AvailableCategories)
	if err != nil {
		return nil, err
	}
	return &cortexv1.CategoryResult{Category: category}, nil
}

func (a *LLMServiceAdapter) FindRelatedFiles(ctx context.Context, req *cortexv1.FindRelatedFilesRequest) (*cortexv1.RelatedFilesResult, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	content := req.GetContent()
	if content == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	related, err := a.handler.FindRelatedFiles(ctx, content, req.CandidateFiles, int(req.MaxResults))
	if err != nil {
		return nil, err
	}
	resp := &cortexv1.RelatedFilesResult{}
	for _, path := range related {
		resp.Files = append(resp.Files, &cortexv1.RelatedFile{RelativePath: path})
	}
	return resp, nil
}

func (a *LLMServiceAdapter) GenerateCompletion(ctx context.Context, req *cortexv1.CompletionRequest) (*cortexv1.CompletionResult, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if req.Prompt == "" {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}
	response, err := a.handler.GenerateCompletion(ctx, llm.GenerateRequest{
		Prompt:      req.Prompt,
		Model:       req.GetModel(),
		MaxTokens:   int(req.MaxTokens),
		Temperature: req.Temperature,
		TimeoutMs:   int(req.TimeoutMs),
	})
	if err != nil {
		return nil, err
	}
	return &cortexv1.CompletionResult{
		Text:             response.Text,
		TokensUsed:       int32(response.TokensUsed),
		ModelUsed:        response.Model,
		ProcessingTimeMs: response.ProcessingTimeMs,
	}, nil
}

func hashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
