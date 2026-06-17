package mcp

import (
	"context"
	"fmt"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultProgressInterval is the default heartbeat interval for progress notifications.
const DefaultProgressInterval = 5 * time.Second

// StartProgressHeartbeat emits MCP progress notifications at the given interval
// for long-running operations. Stops when ctx is cancelled.
// No-op when req or its Session is nil (test mode, stateless transport).
func StartProgressHeartbeat(ctx context.Context, req *sdkmcp.CallToolRequest, message string, interval time.Duration) {
	if req == nil || req.Session == nil {
		return
	}
	start := time.Now()
	token := req.Params.GetProgressToken()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				elapsed := time.Since(start).Round(time.Second)
				_ = req.Session.NotifyProgress(ctx, &sdkmcp.ProgressNotificationParams{
					ProgressToken: token,
					Message:       fmt.Sprintf("%s (elapsed %s)", message, elapsed),
					Progress:      elapsed.Seconds(),
				})
			}
		}
	}()
}
