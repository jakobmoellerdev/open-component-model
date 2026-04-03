package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"ocm.software/open-component-model/mcp/tools"
)

// jsonRPCVersion is the JSON-RPC version used by MCP.
const jsonRPCVersion = "2.0"

// mcpProtocolVersion is the MCP protocol version this server supports.
const mcpProtocolVersion = "2024-11-05"

// Request is a JSON-RPC 2.0 request message.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response message.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC error codes.
const (
	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
	errCodeInternal       = -32603
)

// Server handles MCP protocol messages.
type Server struct {
	toolDeps tools.Deps
}

func newServer(toolDeps tools.Deps) *Server {
	return &Server{toolDeps: toolDeps}
}

// Handle parses a JSON-RPC request and returns the response, or nil for notifications.
func (s *Server) Handle(ctx context.Context, line []byte) *Response {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return &Response{
			JSONRPC: jsonRPCVersion,
			Error:   &RPCError{Code: errCodeParseError, Message: "parse error: " + err.Error()},
		}
	}
	if req.JSONRPC != jsonRPCVersion {
		return &Response{
			JSONRPC: jsonRPCVersion,
			ID:      req.ID,
			Error:   &RPCError{Code: errCodeInvalidRequest, Message: "invalid JSON-RPC version"},
		}
	}

	// Notifications (no ID) — no response needed.
	if req.ID == nil || string(req.ID) == "null" {
		s.handleNotification(ctx, req)
		return nil
	}

	result, rpcErr := s.dispatch(ctx, req)
	if rpcErr != nil {
		return &Response{JSONRPC: jsonRPCVersion, ID: req.ID, Error: rpcErr}
	}
	return &Response{JSONRPC: jsonRPCVersion, ID: req.ID, Result: result}
}

func (s *Server) handleNotification(_ context.Context, req Request) {
	slog.Debug("received notification", "method", req.Method)
}

func (s *Server) dispatch(ctx context.Context, req Request) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "tools/list":
		return s.handleToolsList()
	case "tools/call":
		return s.handleToolsCall(ctx, req.Params)
	case "ping":
		return map[string]any{}, nil
	default:
		return nil, &RPCError{Code: errCodeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

// initializeParams is the parameters for the initialize method.
type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ClientInfo      map[string]any `json:"clientInfo"`
	Capabilities    map[string]any `json:"capabilities"`
}

func (s *Server) handleInitialize(rawParams json.RawMessage) (any, *RPCError) {
	var params initializeParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, &RPCError{Code: errCodeInvalidParams, Message: "invalid initialize params: " + err.Error()}
	}
	return map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "ocm-mcp-server",
			"version": "0.1.0",
		},
	}, nil
}

func (s *Server) handleToolsList() (any, *RPCError) {
	return map[string]any{
		"tools": tools.Definitions(),
	}, nil
}

// toolsCallParams are the parameters for the tools/call method.
type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, rawParams json.RawMessage) (any, *RPCError) {
	var params toolsCallParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, &RPCError{Code: errCodeInvalidParams, Message: "invalid tools/call params: " + err.Error()}
	}

	handler, ok := tools.Registry[params.Name]
	if !ok {
		return nil, &RPCError{Code: errCodeMethodNotFound, Message: fmt.Sprintf("unknown tool: %s", params.Name)}
	}

	text, err := handler(ctx, s.toolDeps, params.Arguments)
	if err != nil {
		// Tool errors are returned as error content, not as JSON-RPC errors.
		return map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("error: %s", err.Error())},
			},
			"isError": true,
		}, nil
	}

	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}, nil
}
