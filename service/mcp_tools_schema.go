package service

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
)

// These schemas preserve the public tool contract of RM-Map-MCP/main.
//
//go:embed mcp_tools_contracts.json
var mcpContracts []byte
var mcpToolDefinitions = func() []mcpToolDefinition {
	var defs []mcpToolDefinition
	if err := json.Unmarshal(mcpContracts, &defs); err != nil {
		panic(err)
	}
	return defs
}()

type mcpToolDefinition struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}
type mcpToolError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status,omitempty"`
}

func (e *mcpToolError) Error() string { return e.Message }
func mcpInvalid(path string) error {
	return &mcpToolError{Code: "invalid_input", Message: "Invalid " + path}
}

// Validate the bounded JSON-schema subset used by the compatibility contracts.
// Unknown properties are stripped, as in Zod.object; strict objects reject them.
func mcpValidate(s map[string]interface{}, v interface{}, path string) (interface{}, error) {
	if options, ok := s["anyOf"].([]interface{}); ok {
		matched := false
		for _, option := range options {
			if _, err := mcpValidate(option.(map[string]interface{}), v, path); err == nil {
				matched = true
				break
			}
		}
		if !matched {
			return nil, mcpInvalid(path)
		}
	}
	if vals, ok := s["enum"].([]interface{}); ok {
		found := false
		for _, a := range vals {
			if a == v {
				found = true
			}
		}
		if !found {
			return nil, mcpInvalid(path)
		}
	}
	switch s["type"] {
	case "object", nil:
		o, ok := v.(map[string]interface{})
		if !ok {
			if s["type"] == nil {
				return v, nil
			}
			return nil, mcpInvalid(path)
		}
		if required, ok := s["required"].([]interface{}); ok {
			for _, key := range required {
				if _, exists := o[key.(string)]; !exists {
					return nil, mcpInvalid(path + "." + key.(string))
				}
			}
		}
		if n, ok := s["minProperties"].(float64); ok && len(o) < int(n) {
			return nil, mcpInvalid(path)
		}
		props, ok := s["properties"].(map[string]interface{})
		if !ok {
			return v, nil
		}
		clean := map[string]interface{}{}
		for key, val := range o {
			schema, exists := props[key]
			if !exists {
				if s["additionalProperties"] == false {
					return nil, mcpInvalid(path + "." + key)
				}
				continue
			}
			c, err := mcpValidate(schema.(map[string]interface{}), val, path+"."+key)
			if err != nil {
				return nil, err
			}
			clean[key] = c
		}
		if n, ok := s["minProperties"].(float64); ok && len(clean) < int(n) {
			return nil, mcpInvalid(path)
		}
		return clean, nil
	case "string":
		str, ok := v.(string)
		if !ok {
			return nil, mcpInvalid(path)
		}
		if n, ok := s["minLength"].(float64); ok && len(str) < int(n) {
			return nil, mcpInvalid(path)
		}
	case "number", "integer":
		n, ok := v.(float64)
		if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, mcpInvalid(path)
		}
		if s["type"] == "integer" && n != math.Trunc(n) {
			return nil, mcpInvalid(path)
		}
		if a, ok := s["minimum"].(float64); ok && n < a {
			return nil, mcpInvalid(path)
		}
		if a, ok := s["exclusiveMinimum"].(float64); ok && n <= a {
			return nil, mcpInvalid(path)
		}
		if a, ok := s["maximum"].(float64); ok && n > a {
			return nil, mcpInvalid(path)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return nil, mcpInvalid(path)
		}
	case "array":
		arr, ok := v.([]interface{})
		if !ok {
			return nil, mcpInvalid(path)
		}
		if n, ok := s["minItems"].(float64); ok && len(arr) < int(n) {
			return nil, mcpInvalid(path)
		}
		clean := make([]interface{}, len(arr))
		for i, item := range arr {
			c, err := mcpValidate(s["items"].(map[string]interface{}), item, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			clean[i] = c
		}
		return clean, nil
	}
	return v, nil
}
