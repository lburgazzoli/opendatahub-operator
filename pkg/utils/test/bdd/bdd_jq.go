package bdd

import (
	"fmt"
	"strings"

	"github.com/itchyny/gojq"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type JQ struct {
	vm *VariableManager
}

func (j *JQ) Matches(in *unstructured.Unstructured, expressions []string) (bool, error) {
	vars := j.vm.GetAll()
	obj := in.DeepCopy()

	vNames := make([]string, 0, len(vars))
	vValues := make([]any, 0, len(vars))

	for k, v := range vars {
		if !strings.HasPrefix(k, "$") {
			k = "$" + k
		}

		vNames = append(vNames, k)
		vValues = append(vValues, v)
	}

	for i := range expressions {
		expression, err := j.vm.Interpolate(expressions[i])
		if err != nil {
			return false, err
		}

		query, err := gojq.Parse(expression)
		if err != nil {
			return false, fmt.Errorf("unable to parse expression %s: %w", expression, err)
		}

		code, err := gojq.Compile(
			query,
			gojq.WithVariables(vNames),
		)
		if err != nil {
			return false, fmt.Errorf("unable to compile expression %s: %w", expression, err)
		}

		it := code.Run(obj.Object, vValues...)

		v, ok := it.Next()
		if !ok {
			return false, nil
		}

		if err, ok := v.(error); ok {
			return false, err
		}

		m, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("unexpected type: %T", v)
		}
		if !m {
			return false, nil
		}
	}

	return true, nil
}

func (j *JQ) Transform(in *unstructured.Unstructured, expressions []string) (*unstructured.Unstructured, error) {
	vars := j.vm.GetAll()
	obj := in.DeepCopy()

	vNames := make([]string, 0, len(vars))
	vValues := make([]any, 0, len(vars))

	for k, v := range vars {
		if !strings.HasPrefix(k, "$") {
			k = "$" + k
		}

		vNames = append(vNames, k)
		vValues = append(vValues, v)
	}

	for i := range expressions {
		expression, err := j.vm.Interpolate(expressions[i])
		if err != nil {
			return obj, err
		}

		query, err := gojq.Parse(expression)
		if err != nil {
			return nil, fmt.Errorf("unable to parse expression %s: %w", expression, err)
		}

		code, err := gojq.Compile(
			query,
			gojq.WithVariables(vNames),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to compile expression %s: %w", expression, err)
		}

		it := code.Run(obj.Object, vValues...)

		v, ok := it.Next()
		if !ok {
			return obj, nil
		}

		if err, ok := v.(error); ok {
			return nil, err
		}

		res, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("result value is not of the expected type (got:%T)", v)
		}

		obj, err = resources.ToUnstructured(&res)
		if err != nil {
			return nil, err
		}
	}

	return obj, nil
}

func (j *JQ) Query(in *unstructured.Unstructured, expression string) (any, error) {
	vars := j.vm.GetAll()
	obj := in.DeepCopy()

	vNames := make([]string, 0, len(vars))
	vValues := make([]any, 0, len(vars))

	for k, v := range vars {
		if !strings.HasPrefix(k, "$") {
			k = "$" + k
		}
		vNames = append(vNames, k)
		vValues = append(vValues, v)
	}

	interpolatedExpression, err := j.vm.Interpolate(expression)
	if err != nil {
		return nil, err
	}

	query, err := gojq.Parse(interpolatedExpression)
	if err != nil {
		return nil, fmt.Errorf("unable to parse expression %s: %w", interpolatedExpression, err)
	}

	code, err := gojq.Compile(
		query,
		gojq.WithVariables(vNames),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to compile expression %s: %w", interpolatedExpression, err)
	}

	it := code.Run(obj.Object, vValues...)

	v, ok := it.Next()
	if !ok {
		return nil, nil
	}

	if err, ok := v.(error); ok {
		return nil, err
	}

	// Convert result to string
	if v == nil {
		return nil, nil
	}

	return v, nil
}
