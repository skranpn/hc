package hc

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	varRegex = regexp.MustCompile(`\{\{(\w+)\}\}`)
	// custom variable (jsonpath)
	symbol           = `[\w\.-_\$\[\]@\*:,\(\)\^\~\<\>=]+`
	jsonpathVarRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9]+\.response\.` + symbol + `)\}\}`)
	// system variable
	systemVarRegex = regexp.MustCompile(`\{\{\$(\w+)\s*(.*)\}\}`)
)

type VariableManager struct {
	variables         map[string]string
	jsonpathVariables map[string]string
}

func NewVariableManager(env map[string]string) *VariableManager {
	if env == nil {
		env = make(map[string]string)
	}

	// Convert env strings to StringValue
	variables := make(map[string]string)
	maps.Copy(variables, env)

	return &VariableManager{
		variables:         variables,
		jsonpathVariables: make(map[string]string),
	}
}

func (vm *VariableManager) Set(key string, value string) {
	vm.variables[key] = vm.ReplaceVariables(value)
}

func (vm *VariableManager) SetJSONPaths(jsonpaths map[string]any) {
	for k, v := range jsonpaths {
		vm.jsonpathVariables[k] = fmt.Sprintf("%v", v)
	}
}

func (vm *VariableManager) Get(key string) string {
	if val, ok := vm.variables[key]; ok {
		return val
	}
	return ""
}

func (vm *VariableManager) FindJSONPath(line string) []string {
	matches := jsonpathVarRegex.FindAllStringSubmatch(line, -1)
	result := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 {
			result = append(result, match[1])
		}
	}

	return result
}

func (vm *VariableManager) ReplaceVariables(input string) string {
	replaced := varRegex.ReplaceAllStringFunc(input, func(query string) string {
		submatches := varRegex.FindStringSubmatch(query)
		if len(submatches) > 1 {
			key := submatches[1]
			if val, ok := vm.variables[key]; ok {
				return val
			}
		}
		return query
	})

	replaced = systemVarRegex.ReplaceAllStringFunc(replaced, func(query string) string {
		submatches := systemVarRegex.FindStringSubmatch(query)
		if len(submatches) > 1 {
			key := submatches[1]
			rest := submatches[2]
			return systemVariables(key, rest)
		}
		return query
	})

	return jsonpathVarRegex.ReplaceAllStringFunc(replaced, func(query string) string {
		submatches := jsonpathVarRegex.FindStringSubmatch(query)
		if len(submatches) > 1 {
			key := submatches[1]
			if val, ok := vm.jsonpathVariables[key]; ok {
				return val
			}
		}
		return query
	})
}

// system variable
// {{$guid}}
// {{$randomInt min max}}
// {{$timestamp [offset option]}}

const (
	// offset_ms = 1
	offset_s = 1
	offset_m = offset_s * 60
	offset_h = offset_m * 60
	offset_d = offset_h * 24
	offset_w = offset_d * 7
	offset_M = offset_d * 30
	offset_y = offset_d * 365
)

var offsetOptionValue = map[string]int64{
	// "ms": offset_ms,
	"s": offset_s,
	"m": offset_m,
	"h": offset_h,
	"d": offset_d,
	"w": offset_w,
	"M": offset_M,
	"y": offset_y,
}

func systemVariables(name string, arg string) string {
	args := strings.Fields(arg)

	switch strings.TrimSpace(name) {
	case "guid":
		return uuid.NewString()

	case "randomInt":
		if len(args) == 0 {
			return fmt.Sprint(rand.Int())
		}

		if len(args) == 1 {
			p1, err := strconv.Atoi(args[0])
			if err != nil {
				return name
			}

			return fmt.Sprint(rand.IntN(max(0, p1)))
		}

		p1, err := strconv.Atoi(args[0])
		if err != nil {
			return name
		}
		p2, err := strconv.Atoi(args[1])
		if err != nil {
			return name
		}

		return fmt.Sprint(rand.IntN(max(p1, p2)-min(p1, p2)+1) + min(p1, p2))

	case "timestamp":
		now := fmt.Sprint(time.Now().Unix())

		if len(args) != 2 {
			return now
		}

		offset, err := strconv.Atoi(args[0])
		if err != nil {
			return now
		}

		option := args[1]
		if !slices.Contains(slices.Collect(maps.Keys(offsetOptionValue)), option) {
			return now
		}

		return fmt.Sprint(time.Now().Unix() + int64(offset)*offsetOptionValue[option])
	}

	return name
}
