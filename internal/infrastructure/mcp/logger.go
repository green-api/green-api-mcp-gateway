// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package mcp

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// generateRequestID creates a new UUID v4 using crypto/rand
func generateRequestID() string {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant bits

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// maskAPIToken replaces api_token values with "***" for security
func maskAPIToken(args map[string]interface{}) map[string]interface{} {
	if args == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	masked := make(map[string]interface{})
	for k, v := range args {
		if strings.Contains(strings.ToLower(k), "api_token") ||
			strings.Contains(strings.ToLower(k), "apitoken") ||
			k == "token" {
			masked[k] = "***"
		} else {
			masked[k] = v
		}
	}
	return masked
}

// getInstanceID extracts instance_id from tool arguments
func getInstanceID(args map[string]interface{}) uint64 {
	if args == nil {
		return 0
	}

	if idVal, ok := args["instance_id"]; ok {
		switch v := idVal.(type) {
		case float64:
			return uint64(v)
		case json.Number:
			if n, err := v.Int64(); err == nil {
				return uint64(n)
			}
		case int:
			return uint64(v)
		case int64:
			return uint64(v)
		case uint64:
			return v
		}
	}
	return 0
}

// calculateResponseSize estimates response size in bytes
func calculateResponseSize(result *mcp.CallToolResult) int {
	if result == nil {
		return 0
	}

	// Rough estimation by marshaling to JSON
	if data, err := json.Marshal(result); err == nil {
		return len(data)
	}
	return 0
}

// logToolRequest logs an MCP tool call request
func logToolRequest(logger *slog.Logger, toolName string, instanceID uint64, args map[string]interface{}, requestID string) {
	logger.Debug("mcp_tool_call",
		slog.String("tool", toolName),
		slog.Uint64("instance_id", instanceID),
		slog.Any("args", maskAPIToken(args)),
		slog.String("request_id", requestID),
	)
}

// logToolResult logs a successful MCP tool call response
func logToolResult(logger *slog.Logger, toolName string, instanceID uint64, result *mcp.CallToolResult, duration time.Duration, requestID string) {
	logger.Debug("mcp_tool_result",
		slog.String("tool", toolName),
		slog.Uint64("instance_id", instanceID),
		slog.String("status", "ok"),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int("response_size", calculateResponseSize(result)),
		slog.String("request_id", requestID),
	)
}

// logToolError logs an MCP tool call error
func logToolError(logger *slog.Logger, toolName string, instanceID uint64, err error, duration time.Duration, requestID string) {
	logger.Warn("mcp_tool_error",
		slog.String("tool", toolName),
		slog.Uint64("instance_id", instanceID),
		slog.String("error", err.Error()),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.String("request_id", requestID),
	)
}

// logRateLimit logs rate limit violations
func logRateLimit(logger *slog.Logger, toolName string, instanceID uint64) {
	logger.Warn("mcp_rate_limit",
		slog.String("tool", toolName),
		slog.Uint64("instance_id", instanceID),
	)
}
