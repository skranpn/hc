package metadata

import (
	"strings"
)

type metadataValue interface{}

type Metadata []metadataValue

func Parse(line string) (metadataValue, error) {
	switch {
	case strings.HasPrefix(line, "assert "):
		// Syntax: assert <jsonPath> <operator> <value>
		line = strings.TrimPrefix(line, "assert ")
		return NewAssertion(line)

	case strings.HasPrefix(line, "until "):
		// Syntax: until <condition> interval <n> max <n>
		line = strings.TrimPrefix(line, "until ")
		return NewUntil(line)
	case strings.HasPrefix(line, "skip "):
		// Syntax: @skip if <condition>
		line = strings.TrimPrefix(line, "skip ")
		return NewSkip(line)
	default:
		// Syntax: @<key> = <value>
		return NewVariable(line)
	}
}

// type MetadataSlice []Metadata

func (m Metadata) OK() bool {
	match := false
	ng := false
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Until:
			match = true
			ng = ng || !v.Condition.Ok()
		case *Assertion:
			match = true
			ng = ng || !v.Ok()
		}
	}

	return match && !ng
}

func (m Metadata) Skipped() bool {
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Skip:
			return v.Condition.Ok()
		}
	}

	return false
}

func (m Metadata) Finish() bool {
	finish := true
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Until:
			finish = finish && v.IsFinish()
		}
	}

	return finish
}

func (m Metadata) Status() string {
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Skip:
			if v.Condition.Ok() {
				return "skipped"
			}
		}
	}

	if m.OK() {
		return "ok"
	}
	return "ng"
}
