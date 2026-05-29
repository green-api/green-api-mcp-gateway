package mcp

import (
	"testing"

	"github.com/green-api/green-api-mcp-gateway/internal/infrastructure"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

func TestRegisteredToolsHaveSubmissionReviewHints(t *testing.T) {
	server := NewServer(infrastructure.NewCredentialManager(), nil, nil, nil, "test")
	tools := server.mcp.ListTools()
	if len(tools) == 0 {
		t.Fatal("expected registered tools")
	}

	for name, entry := range tools {
		annotations := entry.Tool.Annotations
		if annotations.ReadOnlyHint == nil {
			t.Errorf("%s missing readOnlyHint", name)
		}
		if annotations.OpenWorldHint == nil {
			t.Errorf("%s missing openWorldHint", name)
		}
		if annotations.DestructiveHint == nil {
			t.Errorf("%s missing destructiveHint", name)
		}
	}

	assertToolHints(t, tools, "whatsapp_get_contacts", true, false, false)
	assertToolHints(t, tools, "whatsapp_send_message", false, true, true)
	assertToolHints(t, tools, "whatsapp_delete_message", false, true, true)
	assertToolHints(t, tools, "whatsapp_set_settings", false, true, true)
	assertToolHints(t, tools, "whatsapp_disconnect", false, false, false)
}

func TestWidgetToolsExposeResourceTemplateMeta(t *testing.T) {
	server := NewServer(infrastructure.NewCredentialManager(), nil, nil, nil, "test")
	tools := server.mcp.ListTools()

	assertToolResourceURI(t, tools, "whatsapp_get_contacts", "ui://contacts")
	assertToolResourceURI(t, tools, "whatsapp_get_qr", "ui://qr")
}

func assertToolHints(t *testing.T, tools map[string]*mcpgo.ServerTool, name string, readOnly, openWorld, destructive bool) {
	t.Helper()

	entry, ok := tools[name]
	if !ok {
		t.Fatalf("tool %s is not registered", name)
	}
	annotations := entry.Tool.Annotations
	if annotations.ReadOnlyHint == nil || *annotations.ReadOnlyHint != readOnly {
		t.Errorf("%s readOnlyHint = %s, want %v", name, boolPtrString(annotations.ReadOnlyHint), readOnly)
	}
	if annotations.OpenWorldHint == nil || *annotations.OpenWorldHint != openWorld {
		t.Errorf("%s openWorldHint = %s, want %v", name, boolPtrString(annotations.OpenWorldHint), openWorld)
	}
	if annotations.DestructiveHint == nil || *annotations.DestructiveHint != destructive {
		t.Errorf("%s destructiveHint = %s, want %v", name, boolPtrString(annotations.DestructiveHint), destructive)
	}
}

func boolPtrString(value *bool) string {
	if value == nil {
		return "<nil>"
	}
	if *value {
		return "true"
	}
	return "false"
}

func assertToolResourceURI(t *testing.T, tools map[string]*mcpgo.ServerTool, name, want string) {
	t.Helper()

	entry, ok := tools[name]
	if !ok {
		t.Fatalf("tool %s is not registered", name)
	}
	if entry.Tool.Meta == nil {
		t.Fatalf("%s missing _meta", name)
	}
	ui, ok := entry.Tool.Meta.AdditionalFields["ui"].(map[string]any)
	if !ok {
		t.Fatalf("%s missing _meta.ui", name)
	}
	if got := ui["resourceUri"]; got != want {
		t.Fatalf("%s _meta.ui.resourceUri = %v, want %q", name, got, want)
	}
	if got := entry.Tool.Meta.AdditionalFields["openai/outputTemplate"]; got != want {
		t.Fatalf("%s _meta[openai/outputTemplate] = %v, want %q", name, got, want)
	}
}

func TestWidgetResourceMetaHasSubmissionCSPAndDomain(t *testing.T) {
	t.Setenv("GREEN_API_WIDGET_DOMAIN", "https://widgets.green-api.example")

	meta := widgetResourceMeta("Contacts widget", []string{"https://pps.whatsapp.net"})

	ui, ok := meta["ui"].(map[string]any)
	if !ok {
		t.Fatalf("ui metadata missing or wrong type: %#v", meta["ui"])
	}
	if got := ui["domain"]; got != "https://widgets.green-api.example" {
		t.Fatalf("ui.domain = %v, want %q", got, "https://widgets.green-api.example")
	}

	csp, ok := ui["csp"].(map[string]any)
	if !ok {
		t.Fatalf("ui.csp missing or wrong type: %#v", ui["csp"])
	}
	assertStringSlice(t, csp["connectDomains"], []string{"https://api.green-api.com"})
	assertStringSlice(t, csp["resourceDomains"], []string{"https://pps.whatsapp.net"})

	legacyCSP, ok := meta["openai/widgetCSP"].(map[string]any)
	if !ok {
		t.Fatalf("openai/widgetCSP missing or wrong type: %#v", meta["openai/widgetCSP"])
	}
	assertStringSlice(t, legacyCSP["connect_domains"], []string{"https://api.green-api.com"})
	assertStringSlice(t, legacyCSP["resource_domains"], []string{"https://pps.whatsapp.net"})

	if got := meta["openai/widgetDomain"]; got != "https://widgets.green-api.example" {
		t.Fatalf("openai/widgetDomain = %v, want %q", got, "https://widgets.green-api.example")
	}
	if got := meta["openai/widgetDescription"]; got != "Contacts widget" {
		t.Fatalf("openai/widgetDescription = %v, want %q", got, "Contacts widget")
	}
}

func assertStringSlice(t *testing.T, got any, want []string) {
	t.Helper()

	slice, ok := got.([]string)
	if !ok {
		t.Fatalf("value = %#v, want []string", got)
	}
	if len(slice) != len(want) {
		t.Fatalf("len = %d, want %d for %#v", len(slice), len(want), slice)
	}
	for i := range want {
		if slice[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, slice[i], want[i])
		}
	}
}
