package jsonpath

import (
	"encoding/json"
	"fmt"
	"iter"
	"regexp"
	"slices"
	"strings"

	"github.com/theory/jsonpath"
)

func Get(s, path string) (any, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return nil, fmt.Errorf("%s, invalid json, %w", s, err)
	}

	p, err := jsonpath.Parse(serialize(path))
	if err != nil {
		return nil, fmt.Errorf("%s, invalid jsonpath, %w", path, err)
	}

	nodes := p.Select(data).All()
	next, stop := iter.Pull(nodes)
	defer stop()

	v, valid := next()
	if !valid {
		return []any{}, nil
	}

	return v, nil
}

func All(s string, paths []string) (map[string]any, error) {
	results := make(map[string]any)
	for _, path := range paths {
		result, err := Get(s, path)
		if err != nil {
			return nil, err
		}

		results[path] = result
	}

	return results, nil
}

func serialize(path string) string {
	if !isJSONPath(path) {
		return path
	}

	parts := strings.Split(path, ".")
	if parts[0] != "$" {
		parts[0] = "$"
	}

	// headers に続く文字がハイフンを含む場合 ['<name>'] の形にする
	if len(parts) >= 4 && parts[2] == "headers" &&
		strings.Contains(parts[3], "-") &&
		!strings.HasPrefix(parts[3], "[") && !strings.HasSuffix(parts[3], "]") {
		parts[2] = fmt.Sprintf(`%s['%s']`, parts[2], parts[3])

		if len(parts) >= 5 {
			parts = append(parts[:3], parts[4:]...)
		} else {
			parts = parts[:3]
		}
	}

	if len(parts) >= 3 && parts[2] == "body" {
		removed := slices.DeleteFunc(parts[1:], func(v string) bool {
			return v == "$"
		})
		parts = parts[:1+len(removed)]
	}

	return strings.Join(parts, ".")
}

var nameRegex = regexp.MustCompile(`[a-zA-Z0-9]+$`)

func isJSONPath(path string) bool {
	parts := strings.Split(path, ".")
	// at least 3 parts exists, name, response, (status|body|header)
	if len(parts) < 3 {
		return false
	}

	name := parts[0]
	response := parts[1]
	key := parts[2]

	if !nameRegex.MatchString(name) {
		return false
	}
	if response != "response" {
		return false
	}
	if !(key == "status" ||
		key == "body" || strings.HasPrefix(key, "body[") ||
		key == "headers" || strings.HasPrefix(key, "headers[")) {
		return false
	}

	return true
}
