package llm

import (
	"context"
	"time"
)

type traceContextKey struct{}

// TraceInfo links an LLM request to a file and stage.
type TraceInfo struct {
	WorkspaceID   string
	WorkspaceRoot string
	FileID        string
	RelativePath  string
	Stage         string
	Operation     string
}

// LLMTrace captures the prompt/output for a single LLM operation.
type LLMTrace struct {
	Info        TraceInfo
	Prompt      string
	Output      string
	Model       string
	TokensUsed  int
	DurationMs  int64
	Error       *string
	GeneratedAt time.Time
}

// TraceWriter persists LLM traces for inspection.
type TraceWriter interface {
	WriteLLMTrace(ctx context.Context, trace LLMTrace) error
}

// WithTraceInfo attaches LLM trace info to a context.
func WithTraceInfo(ctx context.Context, info TraceInfo) context.Context {
	return context.WithValue(ctx, traceContextKey{}, info)
}

// TraceInfoFromContext extracts LLM trace info from a context.
func TraceInfoFromContext(ctx context.Context) (TraceInfo, bool) {
	info, ok := ctx.Value(traceContextKey{}).(TraceInfo)
	return info, ok
}
