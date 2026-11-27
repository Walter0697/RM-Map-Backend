package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const mcpProtocolVersion = "2025-03-26"
const mcpMaxRequestBytes = 1024 * 1024

type mcpRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPHandler is a stateless Streamable HTTP endpoint. Mount behind MCPAuthenticate.
// It never accepts a client-selected backend URL or an API key as OAuth identity.
func MCPHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if MCPPrincipalForContext(r.Context()) == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
		return
	}
	version := r.Header.Get("MCP-Protocol-Version")
	if version != "" && version != "2024-11-05" && version != "2025-03-26" && version != "2025-06-18" && version != "2025-11-25" {
		http.Error(w, "unsupported MCP protocol version", http.StatusBadRequest)
		return
	}
	body := http.MaxBytesReader(w, r.Body, mcpMaxRequestBytes)
	dec := json.NewDecoder(body)
	var req mcpRPCRequest
	if err := dec.Decode(&req); err != nil {
		mcpRPCError(w, nil, -32700, "Parse error")
		return
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		mcpRPCError(w, nil, -32700, "Only one JSON-RPC request is supported")
		return
	}
	if req.JSONRPC != "2.0" || req.Method == "" || (len(req.ID) > 0 && string(req.ID) != "null" && req.ID[0] != '"' && req.ID[0] != '-' && (req.ID[0] < '0' || req.ID[0] > '9')) {
		mcpRPCError(w, req.ID, -32600, "Invalid Request")
		return
	}
	// Notifications cannot trigger tool execution or any state-changing work.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if json.Unmarshal(req.Params, &p) != nil {
			mcpRPCError(w, req.ID, -32602, "Invalid params")
			return
		}
		selected := mcpProtocolVersion
		if p.ProtocolVersion == "2025-11-25" || p.ProtocolVersion == "2025-06-18" || p.ProtocolVersion == "2024-11-05" {
			selected = p.ProtocolVersion
		}
		mcpRPCResult(w, req.ID, map[string]interface{}{"protocolVersion": selected, "capabilities": map[string]interface{}{"tools": map[string]interface{}{}}, "serverInfo": map[string]string{"name": "roroad-mcp-server", "version": "1.0.0"}})
	case "ping":
		mcpRPCResult(w, req.ID, map[string]interface{}{})
	case "tools/list":
		mcpRPCResult(w, req.ID, map[string]interface{}{"tools": mcpToolDefinitions})
	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if json.Unmarshal(req.Params, &p) != nil {
			mcpRPCError(w, req.ID, -32602, "Invalid params")
			return
		}
		var def *mcpToolDefinition
		for i := range mcpToolDefinitions {
			if mcpToolDefinitions[i].Name == p.Name {
				def = &mcpToolDefinitions[i]
				break
			}
		}
		if def == nil {
			mcpRPCError(w, req.ID, -32602, "Unknown tool")
			return
		}
		if p.Arguments == nil {
			p.Arguments = map[string]interface{}{}
		}
		clean, err := mcpValidate(def.InputSchema, p.Arguments, "arguments")
		var result interface{}
		if err == nil {
			result, err = mcpExecuteTool(r, p.Name, clean.(map[string]interface{}))
		}
		payload := map[string]interface{}{"ok": true, "result": result}
		out := map[string]interface{}{}
		if err != nil {
			e := &mcpToolError{Code: "internal_error", Message: "Unexpected tool error"}
			var typed *mcpToolError
			if errors.As(err, &typed) {
				e = typed
			}
			payload = map[string]interface{}{"ok": false, "error": e}
			out["isError"] = true
		} else if def.OutputSchema != nil {
			// Leave text content untouched for existing MCP clients.
			out["structuredContent"] = payload
		}
		encoded, _ := json.Marshal(payload)
		out["content"] = []map[string]string{{"type": "text", "text": string(encoded)}}
		mcpRPCResult(w, req.ID, out)
	default:
		mcpRPCError(w, req.ID, -32601, "Method not found")
	}
}
func mcpRPCResult(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"jsonrpc": "2.0", "id": id, "result": result})
}
func mcpRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	respondJSON(w, http.StatusOK, map[string]interface{}{"jsonrpc": "2.0", "id": id, "error": map[string]interface{}{"code": code, "message": message}})
}
