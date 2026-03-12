package metadata

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	symbol           = `[\w\.-_\$\[\]@\*:,\(\)\^\~\<\>=]+`
	jsonpathVarRegex = regexp.MustCompile(`\{\{([a-zA-Z0-9]+\.response\.` + symbol + `)\}\}`)
)

type Variable struct {
	Name  string
	Value string

	jsonpaths []string
}

func NewVariable(expr string) (*Variable, error) {
	parts := strings.Fields(expr)
	if len(parts) < 3 {
		return nil, fmt.Errorf("syntax error: @<name> = <value>")
	}

	v := &Variable{
		Name:  parts[0],
		Value: strings.Join(parts[2:], " "),
	}

	for _, match := range jsonpathVarRegex.FindAllStringSubmatch(v.Value, -1) {
		if len(match) > 1 {
			v.jsonpaths = append(v.jsonpaths, match[1])
		}
	}

	return v, nil
}

func (a *Variable) Match(c Cases) error {
	return c.Variable(a)
}

func (v *Variable) JSONPaths() []string {
	return v.jsonpaths
}
