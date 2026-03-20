package metadata

import (
	"fmt"
	"strings"
)

type Skip struct {
	Condition *Assertion
}

func NewSkip(line string) (*Skip, error) {
	parts := strings.Fields(line)
	if parts[0] != "if" {
		return nil, fmt.Errorf("failed to parse skip, syntax: skip if <condition>")
	}

	cond, err := NewAssertion(strings.Join(parts[1:], " "))
	if err != nil {
		return nil, err
	}

	return &Skip{
		Condition: cond,
	}, nil
}
