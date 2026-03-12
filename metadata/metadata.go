package metadata

import (
	"strings"
)

type Metadata interface {
	Match(Cases) error
}

type Cases struct {
	Assertion func(*Assertion) error
	Until     func(*Until) error
	Variable  func(*Variable) error
	Skip      func(*Skip) error
}

func Parse(line string) (Metadata, error) {
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

type MetadataSlice []Metadata

func (m MetadataSlice) OK() bool {
	ng := false
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Until:
			ng = ng || !v.Condition.Ok()
		case *Assertion:
			ng = ng || !v.Ok()
		}
	}

	return !ng
}

func (m MetadataSlice) Skipped() bool {
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Skip:
			return v.Condition.Ok()
		}
	}

	return false
}

func (m MetadataSlice) Finish() bool {
	finish := true
	for _, metadata := range m {
		switch v := metadata.(type) {
		case *Until:
			finish = finish && v.IsFinish()
		}
	}

	return finish
}

func (m MetadataSlice) Status() string {
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
