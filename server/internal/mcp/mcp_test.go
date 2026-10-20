package mcp

import (
	"encoding/json"
	"testing"
)

func TestToolsCatalog(t *testing.T) {
	if len(Tools) != 13 {
		t.Fatalf("expected 13 tools, got %d", len(Tools))
	}

	expectedTools := map[string]bool{
		"save_daily_handoff":        true,
		"get_standup_pending":       true,
		"get_today_note":            true,
		"update_daily_handoff":      true,
		"delete_daily_handoff":      true,
		"add_daily_todo":            true,
		"append_daily_note":         true,
		"list_workspace_bubbles":    true,
		"get_bubble_context":        true,
		"create_workspace_bubble":   true,
		"start_project_server":      true,
		"get_project_server_status": true,
		"stop_project_server":       true,
	}

	for _, tool := range Tools {
		if !expectedTools[tool.Name] {
			t.Errorf("unexpected tool: %s", tool.Name)
		}
		if tool.Description == "" {
			t.Errorf("tool %s is missing description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s is missing input schema", tool.Name)
		}
	}
}

func TestExecuteUnknownTool(t *testing.T) {
	_, err := ExecuteTool("non_existent_tool", nil)
	if err == nil {
		t.Fatal("expected error executing non-existent tool, got nil")
	}
}

func TestJSONRPCSerialization(t *testing.T) {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "initialize",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var parsed JSONRPCRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsed.Method != "initialize" {
		t.Errorf("expected method 'initialize', got %s", parsed.Method)
	}
}

func TestServerToolsValidation(t *testing.T) {
	serverTools := []string{
		"start_project_server",
		"get_project_server_status",
		"stop_project_server",
	}

	for _, name := range serverTools {
		found := false
		for _, tool := range Tools {
			if tool.Name == name {
				found = true
				if tool.Description == "" {
					t.Errorf("tool %s has empty description", name)
				}
				schema, ok := tool.InputSchema.(map[string]interface{})
				if !ok || schema["type"] != "object" {
					t.Errorf("tool %s has invalid schema format", name)
				}
				break
			}
		}
		if !found {
			t.Errorf("tool %s was not found in Tools slice", name)
		}
	}
}

func TestPluginDynamicTools(t *testing.T) {
	// Without plugin enabled, GetTools() returns base Tools
	tools := GetTools()
	if len(tools) != 13 {
		t.Fatalf("expected 13 default tools without plugin, got %d", len(tools))
	}
}
