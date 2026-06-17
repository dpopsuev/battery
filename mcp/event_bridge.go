package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/dpopsuev/battery"
	"github.com/dpopsuev/battery/bus"
	"github.com/dpopsuev/battery/nerve"
	"github.com/dpopsuev/battery/adapter"
	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// EventBridge wraps an MCP server as an EventAdapter.
// Discovers tools via ListTools, subscribes to Motor events for each,
// and publishes Sense events with results.
type EventBridge struct {
	name    string
	tools   []tool.Tool
	session *sdkmcp.ClientSession
	client  *sdkmcp.Client
}

// NewEventBridge connects to an MCP server and discovers its tools.
func NewEventBridge(ctx context.Context, name string, transport sdkmcp.Transport) (*EventBridge, error) {
	clientVersion := "battery/" + battery.Version
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "battery-event-bridge", Version: clientVersion},
		nil,
	)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("event bridge %q connect: %w", name, err)
	}

	if initResult := session.InitializeResult(); initResult != nil && initResult.ServerInfo != nil {
		slog.InfoContext(ctx, "event bridge: MCP connected",
			"adapter", name,
			"server", initResult.ServerInfo.Name,
			"version", initResult.ServerInfo.Version,
		)
	}

	listResult, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		return nil, fmt.Errorf("event bridge %q list tools: %w", name, err)
	}

	tools := make([]tool.Tool, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		var schema json.RawMessage
		if t.InputSchema != nil {
			if data, err := json.Marshal(t.InputSchema); err == nil {
				schema = data
			}
		}
		tools = append(tools, &mcpTool{
			serverName:  name,
			name:        t.Name,
			description: t.Description,
			schema:      schema,
			session:     session,
		})
	}

	return &EventBridge{
		name:    name,
		tools:   tools,
		session: session,
		client:  client,
	}, nil
}

// Name returns the adapter name.
func (b *EventBridge) Name() string { return b.name }

// Description returns a generated description.
func (b *EventBridge) Description() string {
	return fmt.Sprintf("MCP event bridge: %s (%d tools)", b.name, len(b.tools))
}

// Labels returns standard labels.
func (b *EventBridge) Labels() []string { return []string{"mcp", "external"} }

// Tools returns the discovered MCP tools.
func (b *EventBridge) Tools() []tool.Tool { return b.tools }

// Subscriptions returns the Motor event types this adapter handles.
func (b *EventBridge) Subscriptions() adapter.Subscriptions {
	motor := make([]string, 0, len(b.tools))
	for _, t := range b.tools {
		motor = append(motor, t.Name())
	}
	return adapter.Subscriptions{Motor: motor}
}

// Directives returns empty directives.
func (b *EventBridge) Directives() []string { return nil }

// Contributions returns empty contributions.
func (b *EventBridge) Contributions() adapter.Contributions { return adapter.Contributions{} }

// Mount subscribes to Motor events for each tool and publishes Sense results.
func (b *EventBridge) Mount(n nerve.Nerve) func() {
	unsubs := make([]func(), 0, len(b.tools))
	for _, t := range b.tools {
		t := t
		toolName := t.Name()
		unsub := n.Motor().Subscribe(toolName, func(event bus.MotorEvent) {
			result, err := t.Execute(context.Background(), event.Payload)
			if err != nil {
				errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
				n.Sense().Publish(bus.SenseEvent{
					Event:        bus.Event{Type: toolName, CorrelationID: event.CorrelationID},
					Payload:      errPayload,
					IsError:      true,
					ErrorMessage: err.Error(),
					IsFinal:      true,
				})
				return
			}
			payload, _ := json.Marshal(result)
			n.Sense().Publish(bus.SenseEvent{
				Event:   bus.Event{Type: toolName, CorrelationID: event.CorrelationID},
				Payload: payload,
				IsFinal: true,
			})
		})
		unsubs = append(unsubs, unsub)
	}
	return func() {
		for _, u := range unsubs {
			u()
		}
	}
}

// Close disconnects from the MCP server.
func (b *EventBridge) Close() error {
	return b.session.Close()
}
