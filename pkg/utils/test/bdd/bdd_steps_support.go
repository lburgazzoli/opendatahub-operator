package bdd

import (
	"context"
	"errors"
	"fmt"

	"github.com/cucumber/godog"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	gtypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TableToList(table *godog.Table) ([]string, error) {
	values := make([]string, 0, len(table.Rows))

	for i := range table.Rows {
		if table.Rows[i].Cells == nil {
			continue
		}
		if len(table.Rows[i].Cells) != 1 {
			return nil, errors.New("invalid table format")
		}

		values = append(values, table.Rows[i].Cells[0].Value)
	}

	return values, nil
}

func TableToMap(table *godog.Table) (map[string]interface{}, error) {
	if table == nil {
		return make(map[string]interface{}), nil
	}

	result := make(map[string]interface{})

	for i := range table.Rows {
		if table.Rows[i].Cells == nil {
			continue
		}
		if len(table.Rows[i].Cells) != 2 {
			return nil, errors.New("table must have exactly 2 columns (key, value)")
		}

		key := table.Rows[i].Cells[0].Value
		val := table.Rows[i].Cells[1].Value

		result[key] = val
	}

	return result, nil
}

func ConvertToExpressions(in any) ([]string, error) {
	switch v := in.(type) {
	case []string:
		return v, nil
	case string:
		return []string{v}, nil
	case *godog.DocString:
		return []string{v.Content}, nil
	case *godog.Table:
		return TableToList(v)
	default:
		return nil, fmt.Errorf("unsupported expression type: %T", in)
	}
}

func TableToConditionMatchers(tc *TestContext, table *godog.Table) (gstruct.Keys, error) {
	if len(table.Rows) < 2 {
		return nil, errors.New("table must have at least a header row and one data row")
	}

	header := table.Rows[0]
	if len(header.Cells) < 2 {
		return nil, errors.New("table header cannot be empty")
	}
	if header.Cells[0].Value != "type" {
		return nil, errors.New("first column should set the condition type")
	}

	keys := gstruct.Keys{}

	// Process each data row (skip header)
	for i := 1; i < len(table.Rows); i++ {
		row := table.Rows[i]

		fields := gstruct.Keys{}

		for j, cell := range row.Cells {
			fieldName := header.Cells[j].Value
			value, err := tc.Variables().Interpolate(cell.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to interpolate value '%s': %w", cell.Value, err)
			}

			if value != "" {
				switch value {
				case "{{any}}":
					continue
				case "{{nil}}":
					fields[fieldName] = gomega.BeNil()
				case "{{empty}}":
					fields[fieldName] = gomega.BeEmpty()
				case "{{ignore}}":
					continue
				default:
					fields[fieldName] = gomega.Equal(value)
				}
			}
		}

		conditionType := row.Cells[0].Value
		keys[conditionType] = gstruct.MatchKeys(gstruct.IgnoreExtras, fields)
	}

	return keys, nil
}

type Assertion interface {
	Should(matcher gtypes.GomegaMatcher, optionalDescription ...interface{}) bool
	ShouldNot(matcher gtypes.GomegaMatcher, optionalDescription ...interface{}) bool
}

func Assert(
	ctx context.Context,
	t *gomega.WithT,
	mode AssertionMode,
	actual interface{},
	args ...interface{},
) Assertion {
	switch mode {
	case ExpectMode:
		return t.Expect(actual, args...)
	case EventuallyMode:
		return t.Eventually(actual, args...).WithContext(ctx)
	case ConsistentlyMode:
		return t.Consistently(actual, args...).WithContext(ctx)
	default:
		panic(fmt.Errorf("unsupported mode: %s", mode))
	}
}

func Decode(ctx context.Context, content string) (*unstructured.Unstructured, error) {
	tc := TestCtx(ctx)

	interpolatedYAML, err := tc.Variables().Interpolate(content)
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate YAML content: %w", err)
	}

	obj := &unstructured.Unstructured{}
	if _, _, err := tc.Decoder().Decode([]byte(interpolatedYAML), nil, obj); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}

	if obj.GetAPIVersion() == "" {
		return nil, errors.New("YAML must include apiVersion")
	}
	if obj.GetKind() == "" {
		return nil, errors.New("YAML must include kind")
	}
	if obj.GetName() == "" {
		return nil, errors.New("YAML must include metadata.name")
	}

	return obj, nil
}

func InterpolateFromVariable(ctx context.Context, templateVar string, table *godog.Table) (string, error) {
	tc := TestCtx(ctx)

	// Get the template content from variables
	template, exists := tc.Variables().Get(templateVar)
	if !exists {
		return "", fmt.Errorf("template variable '%s' not found", templateVar)
	}

	templateContent, ok := template.(string)
	if !ok {
		return "", fmt.Errorf("template variable '%s' is not a string", templateVar)
	}

	vm := tc.Variables()

	if table != nil {
		// Parse additional values and create copy only when needed
		additionalVars, err := TableToMap(table)
		if err != nil {
			return "", fmt.Errorf("failed to parse values table: %w", err)
		}

		vm = tc.Variables().Copy()
		vm.SetAll(additionalVars)
	}

	interpolatedYAML, err := vm.Interpolate(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to interpolate template '%s': %w", templateVar, err)
	}

	return interpolatedYAML, nil
}
