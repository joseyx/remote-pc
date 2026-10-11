package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ToolCallContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult struct {
	Content []ToolCallContent `json:"content"`
	IsError bool              `json:"isError"`
}

var stdoutMu sync.Mutex

func sendResponse(resp JSONRPCResponse) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling response: %v\n", err)
		return
	}
	_, _ = os.Stdout.Write(data)
	_, _ = os.Stdout.Write([]byte("\n"))
}

// RunServer runs the stdio MCP JSON-RPC 2.0 loop
func RunServer() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimPrefix(line, "\ufeff")
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "\ufeff")
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid JSON-RPC request: %v\n", err)
			continue
		}

		switch req.Method {
		case "initialize":
			sendResponse(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
					"serverInfo": map[string]interface{}{
						"name":    "remote-pc",
						"version": "1.0.0",
					},
				},
			})

		case "notifications/initialized":
			// Notification from client; no response required

		case "ping":
			sendResponse(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  map[string]interface{}{},
			})

		case "tools/list":
			sendResponse(JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]interface{}{
					"tools": Tools,
				},
			})

		case "tools/call":
			var callParams struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &callParams); err != nil {
				sendResponse(JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: ToolCallResult{
						Content: []ToolCallContent{
							{
								Type: "text",
								Text: fmt.Sprintf("Error decoding parameters: %v", err),
							},
						},
						IsError: true,
					},
				})
				continue
			}

			output, execErr := ExecuteTool(callParams.Name, callParams.Arguments)
			if execErr != nil {
				sendResponse(JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: ToolCallResult{
						Content: []ToolCallContent{
							{
								Type: "text",
								Text: fmt.Sprintf("Error: %v", execErr),
							},
						},
						IsError: true,
					},
				})
			} else {
				sendResponse(JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: ToolCallResult{
						Content: []ToolCallContent{
							{
								Type: "text",
								Text: output,
							},
						},
						IsError: false,
					},
				})
			}

		default:
			if len(req.ID) > 0 && string(req.ID) != "null" {
				sendResponse(JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &JSONRPCError{
						Code:    -32601,
						Message: fmt.Sprintf("Method not found: %s", req.Method),
					},
				})
			}
		}
	}

	return nil
}
