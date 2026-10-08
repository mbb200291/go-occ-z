package JsonlOcc

import (
	"fmt"
	"strconv"
	"strings"
)

type pathToken struct {
	key   string
	index int
	array bool
}

// parsePath supports $, dot fields and nonnegative numeric array indexes.
// Wildcards, filters, quoted property names and recursive descent are rejected.
func parsePath(path string) ([]pathToken, error) {
	if strings.TrimSpace(path) == "" || path[0] != '$' {
		return nil, fmt.Errorf("JSON path must start with $: %q", path)
	}
	var tokens []pathToken
	for i := 1; i < len(path); {
		switch path[i] {
		case '.':
			i++
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				i++
			}
			key := path[start:i]
			if key == "" || strings.ContainsAny(key, "*?$] \t\r\n'\"") {
				return nil, fmt.Errorf("unsupported JSON path field in %q", path)
			}
			tokens = append(tokens, pathToken{key: key})
		case '[':
			i++
			start := i
			for i < len(path) && path[i] >= '0' && path[i] <= '9' {
				i++
			}
			if i == start || i >= len(path) || path[i] != ']' {
				return nil, fmt.Errorf("unsupported JSON path index in %q", path)
			}
			index, err := strconv.Atoi(path[start:i])
			if err != nil {
				return nil, fmt.Errorf("invalid JSON path index: %w", err)
			}
			tokens = append(tokens, pathToken{index: index, array: true})
			i++
		default:
			return nil, fmt.Errorf("unsupported JSON path syntax in %q", path)
		}
	}
	return tokens, nil
}

func lookupPath(value any, tokens []pathToken) (any, error) {
	for _, token := range tokens {
		if token.array {
			array, ok := value.([]any)
			if !ok {
				return nil, fmt.Errorf("JSON path index %d requires an array", token.index)
			}
			if token.index >= len(array) {
				return nil, fmt.Errorf("JSON path index %d is out of bounds", token.index)
			}
			value = array[token.index]
		} else {
			object, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("JSON path field %q requires an object", token.key)
			}
			var exists bool
			value, exists = object[token.key]
			if !exists {
				return nil, fmt.Errorf("JSON path field %q does not exist", token.key)
			}
		}
	}
	return value, nil
}
