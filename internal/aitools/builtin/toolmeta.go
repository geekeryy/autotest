package builtin

import (
	"context"
	"encoding/json"

	"autotest/internal/aitools"
)

type toolRun func(ctx context.Context, args json.RawMessage) (any, error)

func readOnlyTool(name, description string, parameters json.RawMessage, run toolRun) aitools.Tool {
	return aitools.Tool{
		Name:        name,
		Description: description,
		Parameters:  parameters,
		Run:         run,
	}
}

func writeTool(name, description string, parameters json.RawMessage, run toolRun) aitools.Tool {
	return aitools.Tool{
		Name:        name,
		Description: description,
		Parameters:  parameters,
		Mutating:    true,
		Run:         run,
	}
}

func deleteTool(name, description string, parameters json.RawMessage, run toolRun) aitools.Tool {
	return aitools.Tool{
		Name:            name,
		Description:     description,
		Parameters:      parameters,
		Mutating:        true,
		RequiresConfirm: true,
		Run:             run,
	}
}
