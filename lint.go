package hc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/skranpn/hc/metadata"
)

type LintIssue struct {
	Severity     LintSeverity
	RequestIndex int
	RequestName  string
	Message      string
}

type LintSeverity string

const (
	LintError   LintSeverity = "error"
	LintWarning LintSeverity = "warning"
)

var bareResponseRefRegex = regexp.MustCompile(`^([a-zA-Z0-9]+)\.response\.`)

func Lint(requests []HttpRequest, envVars map[string]string) []LintIssue {
	var issues []LintIssue

	definedNames := make(map[string]int)
	definedVars := make(map[string]bool)

	for k := range envVars {
		definedVars[k] = true
	}

	for i, req := range requests {
		if req.Name != "" {
			if prev, exists := definedNames[req.Name]; exists {
				issues = append(issues, LintIssue{
					Severity:     LintError,
					RequestIndex: i,
					RequestName:  req.Name,
					Message:      fmt.Sprintf("duplicate @name: %s (also defined at %d)", req.Name, prev+1),
				})
			} else {
				definedNames[req.Name] = i
			}
		}

		for _, m := range req.Metadata {
			switch v := m.(type) {
			case *metadata.Variable:
				definedVars[v.Name] = true
			}
		}
	}

	for i, req := range requests {
		reqName := req.Name

		checkRequestRef := func(refName string) {
			idx, exists := definedNames[refName]
			if !exists {
				issues = append(issues, LintIssue{
					Severity:     LintError,
					RequestIndex: i,
					RequestName:  reqName,
					Message:      fmt.Sprintf("unknown request name: %s", refName),
				})
			} else if idx > i {
				issues = append(issues, LintIssue{
					Severity:     LintWarning,
					RequestIndex: i,
					RequestName:  reqName,
					Message:      fmt.Sprintf("forward reference: %s (defined at #%d)", refName, idx+1),
				})
			}
		}

		checkStr := func(s string) {
			for _, match := range varRegex.FindAllStringSubmatch(s, -1) {
				varName := match[1]
				if !definedVars[varName] {
					issues = append(issues, LintIssue{
						Severity:     LintError,
						RequestIndex: i,
						RequestName:  reqName,
						Message:      fmt.Sprintf("undefined variable: {{%s}}", varName),
					})
				}

			}

			for _, match := range jsonpathVarRegex.FindAllStringSubmatch(s, -1) {
				ref := match[1]
				refName := strings.SplitN(ref, ".", 2)[0]
				checkRequestRef(refName)
			}
		}

		checkBareRef := func(s string) {
			if m := bareResponseRefRegex.FindStringSubmatch(s); m != nil {
				checkRequestRef(m[1])
			}
		}

		checkStr(req.URL)
		for _, v := range req.Headers {
			checkStr(v)
		}
		checkStr(req.Body)

		for _, m := range req.Metadata {
			switch v := m.(type) {
			case *metadata.Assertion:
				checkStr(v.LeftPath)
				checkStr(v.RightValue)
				checkBareRef(v.LeftPath)
				checkBareRef(v.RightValue)
			case *metadata.Until:
				checkStr(v.Condition.LeftPath)
				checkStr(v.Condition.RightValue)
				checkBareRef(v.Condition.LeftPath)
				checkBareRef(v.Condition.RightValue)
			case *metadata.Skip:
				checkStr(v.Condition.LeftPath)
				checkStr(v.Condition.RightValue)
				checkBareRef(v.Condition.LeftPath)
				checkBareRef(v.Condition.RightValue)
			case *metadata.Variable:
				for _, jp := range v.JSONPaths() {
					refName := strings.SplitN(jp, ".", 2)[0]
					checkRequestRef(refName)
				}
			}
		}
	}

	return issues
}
