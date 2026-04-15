package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// Log attribute keys for observable tool calls.
const (
	LogKeyTool    = "tool"
	LogKeyElapsed = "elapsed"
	LogKeyError   = "error"
)

// Observable wraps a Handler with timing and error logging.
func Observable(name string, h Handler) Handler {
	return func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
		start := time.Now()
		result, err := h(ctx, input)
		elapsed := time.Since(start)

		if err != nil {
			slog.WarnContext(ctx, "battery: tool failed",
				slog.String(LogKeyTool, name),
				slog.Duration(LogKeyElapsed, elapsed),
				slog.String(LogKeyError, err.Error()),
			)
		} else {
			slog.DebugContext(ctx, "battery: tool completed",
				slog.String(LogKeyTool, name),
				slog.Duration(LogKeyElapsed, elapsed),
			)
		}
		return result, err
	}
}

// TextResult creates a Result with a single TextContent block.
func TextResult(s string) tool.Result { return tool.TextResult(s) }

// JSONResult creates a Result with StructuredContent and a text fallback.
func JSONResult(data any) (tool.Result, error) { return tool.StructuredResult(data) }
