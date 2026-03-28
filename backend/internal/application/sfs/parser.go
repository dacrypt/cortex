package sfs

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/domain/entity"
)

// ParsedCommand represents a parsed natural language command.
type ParsedCommand struct {
	Operation     OperationType
	Target        string            // Target project, tag, category
	Criteria      map[string]string // Filter criteria
	FileIDs       []entity.FileID   // Specific files to operate on
	Options       map[string]string // Additional options
	RawCommand    string
	Confidence    float64
	Interpretation string // Human-readable interpretation
}

// CommandParser parses natural language commands into structured operations.
type CommandParser struct {
	llmRouter LLMRouter
	patterns  []commandPattern
	logger    zerolog.Logger
}

// commandPattern defines a pattern for matching commands.
type commandPattern struct {
	Pattern   *regexp.Regexp
	Operation OperationType
	Extractor func(matches []string) *ParsedCommand
}

// NewCommandParser creates a new command parser.
func NewCommandParser(llmRouter LLMRouter, logger zerolog.Logger) *CommandParser {
	p := &CommandParser{
		llmRouter: llmRouter,
		logger:    logger.With().Str("component", "sfs-parser").Logger(),
	}
	p.initPatterns()
	return p
}

// initPatterns initializes regex patterns for command parsing.
func (p *CommandParser) initPatterns() {
	p.patterns = []commandPattern{
		// Group commands
		{
			Pattern:   regexp.MustCompile(`(?i)^group\s+(?:all\s+)?(?:files?\s+)?by\s+(\w+)$`),
			Operation: OperationGroup,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationGroup,
					Criteria:  map[string]string{"group_by": m[1]},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^group\s+(?:these\s+)?files?\s+(?:into|as)\s+(.+)$`),
			Operation: OperationGroup,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationGroup,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},

		// Find commands - specific patterns MUST come before generic ones
		{
			Pattern:   regexp.MustCompile(`(?i)^find\s+(?:all\s+)?(?:large|big)\s+files?$`),
			Operation: OperationFind,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationFind,
					Criteria:  map[string]string{"size": "large"},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^find\s+(?:all\s+)?duplicate\s+files?$`),
			Operation: OperationFind,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationFind,
					Criteria:  map[string]string{"duplicates": "true"},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^(?:show|find)\s+(?:all\s+)?unorganized\s+files?$`),
			Operation: OperationFind,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationFind,
					Criteria:  map[string]string{"unorganized": "true"},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^find\s+(?:all\s+)?(\w+)\s+files?$`),
			Operation: OperationFind,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationFind,
					Criteria:  map[string]string{"extension": m[1]},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^find\s+files?\s+(?:modified|changed)\s+(\w+)$`),
			Operation: OperationFind,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationFind,
					Criteria:  map[string]string{"modified": m[1]},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^find\s+(?:files?\s+)?(?:related|similar)\s+to\s+(.+)$`),
			Operation: OperationFind,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationFind,
					Criteria:  map[string]string{"related_to": strings.TrimSpace(m[1])},
				}
			},
		},

		// Tag commands
		{
			Pattern:   regexp.MustCompile(`(?i)^tag\s+(?:these\s+)?(?:files?\s+)?(?:as|with)\s+(.+)$`),
			Operation: OperationTag,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationTag,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^add\s+tag\s+(.+?)(?:\s+to\s+(?:these\s+)?files?)?$`),
			Operation: OperationTag,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationTag,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},

		// Untag commands
		{
			Pattern:   regexp.MustCompile(`(?i)^(?:untag|remove\s+tag)\s+(.+?)(?:\s+from\s+(?:these\s+)?files?)?$`),
			Operation: OperationUntag,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationUntag,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},

		// Assign commands
		{
			Pattern:   regexp.MustCompile(`(?i)^assign\s+(?:these\s+)?(?:files?\s+)?to\s+(?:project\s+)?(.+)$`),
			Operation: OperationAssign,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationAssign,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^(?:add|move)\s+(?:these\s+)?(?:files?\s+)?to\s+(?:project\s+)?(.+)$`),
			Operation: OperationAssign,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationAssign,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},

		// Unassign commands
		{
			Pattern:   regexp.MustCompile(`(?i)^(?:unassign|remove)\s+(?:these\s+)?(?:files?\s+)?from\s+(?:project\s+)?(.+)$`),
			Operation: OperationUnassign,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationUnassign,
					Target:    strings.TrimSpace(m[1]),
				}
			},
		},

		// Create commands - specific patterns MUST come before generic ones
		{
			Pattern:   regexp.MustCompile(`(?i)^create\s+project\s+(?:from|for)\s+(?:this\s+)?folder$`),
			Operation: OperationCreate,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationCreate,
					Criteria:  map[string]string{"from": "folder"},
				}
			},
		},
		{
			Pattern:   regexp.MustCompile(`(?i)^create\s+(?:a\s+)?(?:new\s+)?project\s+(?:called\s+|named\s+)?(.+)$`),
			Operation: OperationCreate,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationCreate,
					Target:    strings.TrimSpace(m[1]),
					Criteria:  map[string]string{"type": "project"},
				}
			},
		},

		// Merge commands
		{
			Pattern:   regexp.MustCompile(`(?i)^merge\s+(?:projects?\s+)?(.+?)\s+(?:into|with)\s+(.+)$`),
			Operation: OperationMerge,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationMerge,
					Target:    strings.TrimSpace(m[2]),
					Criteria:  map[string]string{"source": strings.TrimSpace(m[1])},
				}
			},
		},

		// Rename commands
		{
			Pattern:   regexp.MustCompile(`(?i)^rename\s+(?:project\s+)?(.+?)\s+to\s+(.+)$`),
			Operation: OperationRename,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationRename,
					Target:    strings.TrimSpace(m[2]),
					Criteria:  map[string]string{"from": strings.TrimSpace(m[1])},
				}
			},
		},

		// Summarize commands
		{
			Pattern:   regexp.MustCompile(`(?i)^summarize\s+(?:these\s+)?(?:files?|documents?)$`),
			Operation: OperationSummarize,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationSummarize,
				}
			},
		},

		// Relate commands
		{
			Pattern:   regexp.MustCompile(`(?i)^(?:relate|link)\s+(?:these\s+)?files?$`),
			Operation: OperationRelate,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationRelate,
				}
			},
		},

		// Query commands
		{
			Pattern:   regexp.MustCompile(`(?i)^(?:what|which|how|where|when|why|who)\s+.+\?$`),
			Operation: OperationQuery,
			Extractor: func(m []string) *ParsedCommand {
				return &ParsedCommand{
					Operation: OperationQuery,
					Criteria:  map[string]string{"query": m[0]},
				}
			},
		},
	}
}

// Parse parses a natural language command.
func (p *CommandParser) Parse(ctx context.Context, command string, contextFileIDs []entity.FileID) (*ParsedCommand, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, fmt.Errorf("empty command")
	}

	// Try pattern matching first
	parsed := p.parseWithPatterns(command)
	if parsed != nil {
		parsed.RawCommand = command
		parsed.FileIDs = contextFileIDs
		parsed.Confidence = 0.9
		parsed.Interpretation = p.generateInterpretation(parsed)
		return parsed, nil
	}

	// Fall back to LLM parsing if available
	if p.llmRouter != nil {
		parsed, err := p.parseWithLLM(ctx, command, contextFileIDs)
		if err != nil {
			p.logger.Debug().Err(err).Str("command", command).Msg("LLM parsing failed")
		} else if parsed != nil {
			return parsed, nil
		}
	}

	// Return unknown operation
	return &ParsedCommand{
		Operation:      OperationUnknown,
		RawCommand:     command,
		FileIDs:        contextFileIDs,
		Confidence:     0.3,
		Interpretation: fmt.Sprintf("Could not understand command: %s", command),
	}, nil
}

// parseWithPatterns attempts to parse using regex patterns.
func (p *CommandParser) parseWithPatterns(command string) *ParsedCommand {
	for _, pattern := range p.patterns {
		matches := pattern.Pattern.FindStringSubmatch(command)
		if matches != nil {
			parsed := pattern.Extractor(matches)
			parsed.Operation = pattern.Operation
			return parsed
		}
	}
	return nil
}

// parseWithLLM uses the LLM to parse complex commands.
func (p *CommandParser) parseWithLLM(ctx context.Context, command string, contextFileIDs []entity.FileID) (*ParsedCommand, error) {
	prompt := p.buildParsingPrompt(command, len(contextFileIDs))

	response, err := p.llmRouter.Complete(ctx, prompt, 500)
	if err != nil {
		return nil, err
	}

	return p.parseLLMResponse(command, response, contextFileIDs)
}

// buildParsingPrompt builds the LLM prompt for command parsing.
func (p *CommandParser) buildParsingPrompt(command string, fileCount int) string {
	return fmt.Sprintf(`Parse this file organization command and extract the operation and parameters.

Command: "%s"
Context: %d files selected

Available operations:
- group: Organize files into projects/categories
- find: Search for files matching criteria
- tag: Add tags to files
- untag: Remove tags from files
- assign: Assign files to a project
- unassign: Remove files from a project
- create: Create a new project/category
- merge: Merge projects/categories
- rename: Rename a project/category
- summarize: Generate summary for files
- relate: Create relationships between files
- query: Ask a question about files

Output format (one line):
OPERATION|target|criteria_key1=value1,criteria_key2=value2|confidence

Example:
group|Documents|group_by=type|0.85
find||extension=pdf,modified=today|0.9
tag|important||0.95
`, command, fileCount)
}

// parseLLMResponse parses the LLM response into a ParsedCommand.
func (p *CommandParser) parseLLMResponse(command, response string, contextFileIDs []entity.FileID) (*ParsedCommand, error) {
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "Example:") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 1 {
			continue
		}

		operation := parseOperationType(strings.TrimSpace(parts[0]))
		if operation == OperationUnknown {
			continue
		}

		parsed := &ParsedCommand{
			Operation:  operation,
			RawCommand: command,
			FileIDs:    contextFileIDs,
			Criteria:   make(map[string]string),
			Options:    make(map[string]string),
			Confidence: 0.7,
		}

		if len(parts) > 1 {
			parsed.Target = strings.TrimSpace(parts[1])
		}

		if len(parts) > 2 {
			criteriaStr := strings.TrimSpace(parts[2])
			for _, kv := range strings.Split(criteriaStr, ",") {
				if idx := strings.Index(kv, "="); idx > 0 {
					key := strings.TrimSpace(kv[:idx])
					value := strings.TrimSpace(kv[idx+1:])
					parsed.Criteria[key] = value
				}
			}
		}

		if len(parts) > 3 {
			if conf, err := parseFloat(parts[3]); err == nil {
				parsed.Confidence = conf
			}
		}

		parsed.Interpretation = p.generateInterpretation(parsed)
		return parsed, nil
	}

	return nil, fmt.Errorf("could not parse LLM response")
}

// generateInterpretation generates a human-readable interpretation.
func (p *CommandParser) generateInterpretation(cmd *ParsedCommand) string {
	switch cmd.Operation {
	case OperationGroup:
		if groupBy, ok := cmd.Criteria["group_by"]; ok {
			return fmt.Sprintf("Group files by %s", groupBy)
		}
		if cmd.Target != "" {
			return fmt.Sprintf("Group files into '%s'", cmd.Target)
		}
		return "Group files"

	case OperationFind:
		var conditions []string
		if ext, ok := cmd.Criteria["extension"]; ok {
			conditions = append(conditions, fmt.Sprintf("with extension .%s", ext))
		}
		if mod, ok := cmd.Criteria["modified"]; ok {
			conditions = append(conditions, fmt.Sprintf("modified %s", mod))
		}
		if _, ok := cmd.Criteria["unorganized"]; ok {
			conditions = append(conditions, "not in any project")
		}
		if _, ok := cmd.Criteria["duplicates"]; ok {
			conditions = append(conditions, "that are duplicates")
		}
		if rel, ok := cmd.Criteria["related_to"]; ok {
			conditions = append(conditions, fmt.Sprintf("related to '%s'", rel))
		}
		if size, ok := cmd.Criteria["size"]; ok {
			conditions = append(conditions, fmt.Sprintf("that are %s", size))
		}
		if len(conditions) > 0 {
			return fmt.Sprintf("Find files %s", strings.Join(conditions, " and "))
		}
		return "Find files"

	case OperationTag:
		return fmt.Sprintf("Add tag '%s' to files", cmd.Target)

	case OperationUntag:
		return fmt.Sprintf("Remove tag '%s' from files", cmd.Target)

	case OperationAssign:
		return fmt.Sprintf("Assign files to project '%s'", cmd.Target)

	case OperationUnassign:
		return fmt.Sprintf("Remove files from project '%s'", cmd.Target)

	case OperationCreate:
		if cmd.Target != "" {
			return fmt.Sprintf("Create new project '%s'", cmd.Target)
		}
		if from, ok := cmd.Criteria["from"]; ok {
			return fmt.Sprintf("Create project from %s", from)
		}
		return "Create new project"

	case OperationMerge:
		source := cmd.Criteria["source"]
		return fmt.Sprintf("Merge '%s' into '%s'", source, cmd.Target)

	case OperationRename:
		from := cmd.Criteria["from"]
		return fmt.Sprintf("Rename '%s' to '%s'", from, cmd.Target)

	case OperationSummarize:
		return "Generate AI summary for files"

	case OperationRelate:
		return "Create relationships between files"

	case OperationQuery:
		return fmt.Sprintf("Answer question: %s", cmd.Criteria["query"])

	default:
		return fmt.Sprintf("Unknown operation: %s", cmd.RawCommand)
	}
}

// parseOperationType converts a string to OperationType.
func parseOperationType(s string) OperationType {
	switch strings.ToLower(s) {
	case "group":
		return OperationGroup
	case "find":
		return OperationFind
	case "tag":
		return OperationTag
	case "untag":
		return OperationUntag
	case "assign":
		return OperationAssign
	case "unassign":
		return OperationUnassign
	case "create":
		return OperationCreate
	case "merge":
		return OperationMerge
	case "rename":
		return OperationRename
	case "summarize":
		return OperationSummarize
	case "relate":
		return OperationRelate
	case "query":
		return OperationQuery
	default:
		return OperationUnknown
	}
}

func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
