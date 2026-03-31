package main

import (
	"encoding/json"
	"fmt"
)

const maxFieldBytes = 10 * 1024 // 10KB

// truncateJSON returns the JSON string of value, truncated to maxFieldBytes if needed.
// If the encoded JSON exceeds the limit, it returns a JSON object with
// _truncated, _original_size, and content fields.
func truncateJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `null`
	}
	if len(raw) <= maxFieldBytes {
		return string(raw)
	}
	return truncateRaw(raw)
}

// truncateRawJSON truncates a raw JSON string (already encoded) to maxFieldBytes.
func truncateRawJSON(raw string) string {
	if len(raw) <= maxFieldBytes {
		return raw
	}
	return truncateRaw([]byte(raw))
}

func truncateRaw(raw []byte) string {
	originalSize := len(raw)
	// Reserve space for the wrapper object overhead
	meta := fmt.Sprintf(`{"_truncated":true,"_original_size":%d,"content":""}`, originalSize)
	overhead := len(meta)
	budget := maxFieldBytes - overhead
	if budget < 0 {
		budget = 0
	}

	// Take budget bytes, ensuring valid UTF-8 boundary
	content := string(raw[:budget])

	result := map[string]any{
		"_truncated":     true,
		"_original_size": originalSize,
		"content":        content,
	}
	out, _ := json.Marshal(result)

	// If JSON escaping caused the result to exceed maxFieldBytes, trim content
	for len(out) > maxFieldBytes && len(content) > 0 {
		content = content[:len(content)-1]
		result["content"] = content
		out, _ = json.Marshal(result)
	}
	return string(out)
}
