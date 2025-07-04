package jq

import (
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"gopkg.in/yaml.v3"
)

type variable struct {
	key string
	val any
}

type Builder struct {
	variables map[string]interface{}
}

func (b Builder) Match(format string, args ...any) *Matcher {
	variables := make([]variable, 0, len(b.variables))

	for k, v := range b.variables {
		if !strings.HasPrefix(k, "$") {
			k = "$" + k
		}

		variables = append(variables, variable{key: k, val: v})
	}

	return &Matcher{
		expression: fmt.Sprintf(format, args...),
		variables:  variables,
	}
}

func WithVariables(vars map[string]any) Builder {
	return Builder{variables: vars}
}

func Match(format string, args ...any) *Matcher {
	return &Matcher{
		expression: fmt.Sprintf(format, args...),
	}
}

var _ types.GomegaMatcher = &Matcher{}

type Matcher struct {
	expression       string
	variables        []variable
	firstFailurePath []interface{}
}

func (matcher *Matcher) Match(actual interface{}) (bool, error) {
	query, err := gojq.Parse(matcher.expression)
	if err != nil {
		return false, gomega.StopTrying(fmt.Sprintf("unable to parse expression %s", matcher.expression)).Wrap(err)
	}

	vNames := make([]string, len(matcher.variables))
	for i := range matcher.variables {
		vNames[i] = matcher.variables[i].key
	}

	vValues := make([]any, len(matcher.variables))
	for i := range matcher.variables {
		vValues[i] = matcher.variables[i].val
	}

	code, err := gojq.Compile(
		query,
		gojq.WithVariables(vNames),
	)
	if err != nil {
		return false, gomega.StopTrying(fmt.Sprintf("unable to compile expression %s", matcher.expression)).Wrap(err)
	}

	data, err := toType(actual)
	if err != nil {
		return false, gomega.StopTrying("unable to convert input to a supported type").Wrap(err)
	}

	it := code.Run(data, vValues...)

	v, ok := it.Next()
	if !ok {
		return false, nil
	}

	if err, ok := v.(error); ok {
		return false, err
	}

	if match, ok := v.(bool); ok {
		return match, nil
	}

	return false, nil
}

func (matcher *Matcher) FailureMessage(actual interface{}) string {
	content, err := yaml.Marshal(actual)
	if err != nil {
		panic(err)
	}

	msg := format.Message(string(content), "to match expression", matcher.expression)

	return formattedMessage(msg, matcher.firstFailurePath)
}

func (matcher *Matcher) NegatedFailureMessage(actual interface{}) string {
	content, err := yaml.Marshal(actual)
	if err != nil {
		panic(err)
	}

	msg := format.Message(string(content), "not to match expression", matcher.expression)

	return formattedMessage(msg, matcher.firstFailurePath)
}
