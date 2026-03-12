package metadata

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/skranpn/hc/jsonpath"
)

var AssertOps = map[string]bool{
	"<=":       false,
	">=":       false,
	"!=":       false,
	"==":       false,
	"<":        false,
	">":        false,
	"contains": false,
	"is":       false,
}

type Assertion struct {
	Raw        string // for logging
	Operator   string // operator like <=, >, contains
	LeftPath   string // JSONPath
	RightValue string // can be string or number

	ok bool
}

func NewAssertion(line string) (*Assertion, error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil, fmt.Errorf("syntax error: @assert <left> <op> <right>")
	}
	left := parts[0]
	op := parts[1]
	right := strings.Join(parts[2:], " ")

	if _, ok := AssertOps[op]; !ok {
		return nil, fmt.Errorf(
			"syntax error: assert operator should be one of %s",
			strings.Join(slices.Collect(maps.Keys(AssertOps)), ","),
		)
	}

	return &Assertion{
		Raw:        line,
		LeftPath:   left,
		Operator:   op,
		RightValue: right,
	}, nil
}

// Assertion is Metadata
func (a *Assertion) Match(c Cases) error {
	return c.Assertion(a)
}

func (a *Assertion) StatusText() string {
	if a.ok {
		return "ok"
	}
	return "ng"
}

func (a *Assertion) Ok() bool {
	return a.ok
}

func (a *Assertion) Evaluate(unifiedJson string) (result bool, err error) {
	defer func() {
		a.ok = result
	}()

	left, err := jsonpath.Get(unifiedJson, a.LeftPath)
	if err != nil {
		left = a.LeftPath
	}

	right, err := jsonpath.Get(unifiedJson, a.RightValue)
	if err != nil {
		right = a.RightValue
	}

	// Evaluate based on operator
	switch a.Operator {
	case "==":
		return a.compareEqual(left, right)
	case "!=":
		eq, err := a.compareEqual(left, right)
		return !eq, err
	case "<", ">", "<=", ">=":
		return a.compareNumeric(left, right)
	case "contains":
		return a.compareContains(left, right)
	case "is":
		return a.compareIs(left, right)
	default:
		return false, fmt.Errorf("unsupported operator: %s", a.Operator)
	}
}

func (a *Assertion) compareEqual(left, right any) (bool, error) {
	return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right), nil
}

func (a *Assertion) compareNumeric(left, right any) (bool, error) {
	leftFloat, err := strconv.ParseFloat(fmt.Sprintf("%v", left), 64)
	if err != nil {
		return false, fmt.Errorf("left side is not numeric: %v", left)
	}

	rightFloat, err := strconv.ParseFloat(fmt.Sprintf("%v", right), 64)
	if err != nil {
		return false, fmt.Errorf("right side is not numeric: %v", right)
	}

	switch a.Operator {
	case "<":
		return leftFloat < rightFloat, nil
	case ">":
		return leftFloat > rightFloat, nil
	case "<=":
		return leftFloat <= rightFloat, nil
	case ">=":
		return leftFloat >= rightFloat, nil
	}
	return false, fmt.Errorf("unknown numeric operator: %s", a.Operator)
}

func (a *Assertion) compareContains(left, right any) (bool, error) {
	return strings.Contains(fmt.Sprintf("%v", left), fmt.Sprintf("%v", right)), nil
}

func (a *Assertion) compareIs(left, right any) (bool, error) {
	rightStr := fmt.Sprintf("%v", right)
	leftStr := fmt.Sprintf("%v", left)

	switch strings.ToLower(rightStr) {
	case "array":
		b, err := json.Marshal(left)
		if err != nil {
			return false, nil
		}

		var arr []any
		return json.Unmarshal(b, &arr) == nil, nil
	case "object":
		if left == nil {
			return false, nil
		}

		b, err := json.Marshal(left)
		if err != nil {
			return false, nil
		}

		var obj map[string]any
		return json.Unmarshal(b, &obj) == nil, nil
	case "null":
		return leftStr == "null" || leftStr == "<nil>", nil
	case "string":
		_, ok := left.(string)
		return ok, nil
	case "number":
		_, err := strconv.ParseFloat(leftStr, 64)
		return err == nil, nil
	case "bool", "boolean":
		_, err := strconv.ParseBool(leftStr)
		return err == nil, nil
	default:
		return false, fmt.Errorf("unknown 'is' operator: %s", rightStr)
	}
}
