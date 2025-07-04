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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
)

type AssertionMode string

const (
	ExpectMode       AssertionMode = "expect"
	EventuallyMode   AssertionMode = "eventually"
	ConsistentlyMode AssertionMode = "consistently"
)

const TestFieldOwner = "bdd-test.opendatahub.io"

func InitializeResourceSteps(ctx *godog.ScenarioContext) {
	initializeGeneralResourceSteps(ctx)
	initializeNamespacedResourceSteps(ctx)
	initializeClusterResourceSteps(ctx)
	initializeResourceTypeSteps(ctx)
}

func initializeGeneralResourceSteps(ctx *godog.ScenarioContext) {
	// Resource creation
	ctx.Step(
		"^I create the resource:$",
		func(ctx context.Context, yaml *godog.DocString) error {
			return CreateResourceFromYAML(ctx, yaml.Content)
		})

	// Resource application (server-side apply)
	ctx.Step(
		"^I apply the resource:$",
		func(ctx context.Context, yaml *godog.DocString) error {
			return ApplyResourceFromYAML(ctx, yaml.Content)
		})

	// Resource creation from template
	ctx.Step(
		"^I create the resource from template `([^`]+)`$",
		func(ctx context.Context, templateVar string) error {
			return CreateResourceFromTemplate(ctx, templateVar, nil)
		})

	ctx.Step(
		"^I create the resource from template `([^`]+)` with values:$",
		func(ctx context.Context, templateVar string, table *godog.Table) error {
			return CreateResourceFromTemplate(ctx, templateVar, table)
		})

	// Resource application from template
	ctx.Step(
		"^I apply the resource from template `([^`]+)`$",
		func(ctx context.Context, templateVar string) error {
			return ApplyResourceFromTemplate(ctx, templateVar, nil)
		})

	ctx.Step(
		"^I apply the resource from template `([^`]+)` with values:$",
		func(ctx context.Context, templateVar string, table *godog.Table) error {
			return ApplyResourceFromTemplate(ctx, templateVar, table)
		})
}

func initializeNamespacedResourceSteps(ctx *godog.ScenarioContext) {

	// Resource existence
	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` in namespace `([^`]+)` should exist",
		func(ctx context.Context, mode string, resourceType string, name string, namespace string) error {
			return Resource(ctx, AssertionMode(mode), resourceType, name, namespace, gomega.Not(gomega.BeNil()))
		})

	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` in namespace `([^`]+)` should not exist",
		func(ctx context.Context, mode string, resourceType string, name string, namespace string) error {
			return Resource(ctx, AssertionMode(mode), resourceType, name, namespace, gomega.BeNil())
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` in namespace `([^`]+)` to exist",
		func(ctx context.Context, resourceType string, name string, namespace string) error {
			return Resource(ctx, ExpectMode, resourceType, name, namespace, gomega.Not(gomega.BeNil()))
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` in namespace `([^`]+)` to not exist",
		func(ctx context.Context, resourceType string, name string, namespace string) error {
			return Resource(ctx, ExpectMode, resourceType, name, namespace, gomega.BeNil())
		})

	// Expression matching
	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` in namespace `([^`]+)` matches expression `([^`]+)`",
		func(ctx context.Context, mode string, resourceType string, name string, namespace string, expression string) error {
			return ResourceMatchesExpressions(ctx, AssertionMode(mode), resourceType, name, namespace, expression)
		})

	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` in namespace `([^`]+)` matches expression:$",
		func(ctx context.Context, mode string, resourceType string, name string, namespace string, expression *godog.DocString) error {
			return ResourceMatchesExpressions(ctx, AssertionMode(mode), resourceType, name, namespace, expression)
		})

	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` in namespace `([^`]+)` matches expressions:$",
		func(ctx context.Context, mode string, resourceType string, name string, namespace string, table *godog.Table) error {
			return ResourceMatchesExpressions(ctx, AssertionMode(mode), resourceType, name, namespace, table)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` in namespace `([^`]+)` to match expression `([^`]+)`",
		func(ctx context.Context, resourceType string, name string, namespace string, expression string) error {
			return ResourceMatchesExpressions(ctx, ExpectMode, resourceType, name, namespace, expression)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` in namespace `([^`]+)` to match expression:$",
		func(ctx context.Context, resourceType string, name string, namespace string, expression *godog.DocString) error {
			return ResourceMatchesExpressions(ctx, ExpectMode, resourceType, name, namespace, expression)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` in namespace `([^`]+)` to match expressions:$",
		func(ctx context.Context, resourceType string, name string, namespace string, table *godog.Table) error {
			return ResourceMatchesExpressions(ctx, ExpectMode, resourceType, name, namespace, table)
		})

	// List matching with selectors
	ctx.Step(
		"^(eventually|consistently) `([^`]+)` in namespace `([^`]+)` with selector `([^`]+)` should match expression `([^`]+)`",
		func(ctx context.Context, mode string, resourceType string, namespace string, selector string, expression string) error {
			return ResourceListMatchesExpressions(ctx, AssertionMode(mode), resourceType, namespace, selector, expression)
		})

	ctx.Step(
		"^(eventually|consistently) `([^`]+)` in namespace `([^`]+)` with selector `([^`]+)` should match expression:$",
		func(ctx context.Context, mode string, resourceType string, namespace string, selector string, expression *godog.DocString) error {
			return ResourceListMatchesExpressions(ctx, AssertionMode(mode), resourceType, namespace, selector, expression)
		})

	ctx.Step(
		"^(eventually|consistently) `([^`]+)` in namespace `([^`]+)` with selector `([^`]+)` should match expressions:$",
		func(ctx context.Context, mode string, resourceType string, namespace string, selector string, table *godog.Table) error {
			return ResourceListMatchesExpressions(ctx, AssertionMode(mode), resourceType, namespace, selector, table)
		})

	ctx.Step(
		"^expect `([^`]+)` in namespace `([^`]+)` with selector `([^`]+)` to match expression `([^`]+)`",
		func(ctx context.Context, resourceType string, namespace string, selector string, expression string) error {
			return ResourceListMatchesExpressions(ctx, ExpectMode, resourceType, namespace, selector, expression)
		})

	ctx.Step(
		"^expect `([^`]+)` in namespace `([^`]+)` with selector `([^`]+)` to match expression:$",
		func(ctx context.Context, resourceType string, namespace string, selector string, expression *godog.DocString) error {
			return ResourceListMatchesExpressions(ctx, ExpectMode, resourceType, namespace, selector, expression)
		})

	ctx.Step(
		"^expect `([^`]+)` in namespace `([^`]+)` with selector `([^`]+)` to match expressions:$",
		func(ctx context.Context, resourceType string, namespace string, selector string, table *godog.Table) error {
			return ResourceListMatchesExpressions(ctx, ExpectMode, resourceType, namespace, selector, table)
		})

	// Resource deletion
	ctx.Step(
		"^I delete the `([^`]+)` `([^`]+)` in namespace `([^`]+)`",
		func(ctx context.Context, resourceType string, name string, namespace string) error {
			return DeleteResource(ctx, resourceType, name, namespace)
		})

	// Resource updates
	ctx.Step(
		"^I update the `([^`]+)` `([^`]+)` in namespace `([^`]+)` with expression `([^`]+)`",
		func(ctx context.Context, resourceType string, name string, namespace string, expression string) error {
			return UpdateResourceWithExpressions(ctx, resourceType, name, namespace, expression)
		})

	ctx.Step(
		"^I update the `([^`]+)` `([^`]+)` in namespace `([^`]+)` with expression:$",
		func(ctx context.Context, resourceType string, name string, namespace string, expression *godog.DocString) error {
			return UpdateResourceWithExpressions(ctx, resourceType, name, namespace, expression)
		})

	ctx.Step(
		"^I update the `([^`]+)` `([^`]+)` in namespace `([^`]+)` with expressions:$",
		func(ctx context.Context, resourceType string, name string, namespace string, table *godog.Table) error {
			return UpdateResourceWithExpressions(ctx, resourceType, name, namespace, table)
		})

	// Conditions
	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` in namespace `([^`]+)` has conditions:$",
		func(ctx context.Context, mode string, resourceType string, name string, namespace string, table *godog.Table) error {
			return ResourceHasConditions(ctx, AssertionMode(mode), resourceType, name, namespace, table)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` in namespace `([^`]+)` to have conditions:$",
		func(ctx context.Context, resourceType string, name string, namespace string, table *godog.Table) error {
			return ResourceHasConditions(ctx, ExpectMode, resourceType, name, namespace, table)
		})
}

func initializeClusterResourceSteps(ctx *godog.ScenarioContext) {
	// Resource existence
	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` should exist",
		func(ctx context.Context, mode string, resourceType string, name string) error {
			return Resource(ctx, AssertionMode(mode), resourceType, name, "", gomega.Not(gomega.BeNil()))
		})

	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` should not exist",
		func(ctx context.Context, mode string, resourceType string, name string) error {
			return Resource(ctx, AssertionMode(mode), resourceType, name, "", gomega.BeNil())
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` to exist",
		func(ctx context.Context, resourceType string, name string) error {
			return Resource(ctx, ExpectMode, resourceType, name, "", gomega.Not(gomega.BeNil()))
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` to not exist",
		func(ctx context.Context, resourceType string, name string) error {
			return Resource(ctx, ExpectMode, resourceType, name, "", gomega.BeNil())
		})

	// Expression matching
	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` matches expression `([^`]+)`",
		func(ctx context.Context, mode string, resourceType string, name string, expression string) error {
			return ResourceMatchesExpressions(ctx, AssertionMode(mode), resourceType, name, "", expression)
		})

	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` matches expression:$",
		func(ctx context.Context, mode string, resourceType string, name string, expression *godog.DocString) error {
			return ResourceMatchesExpressions(ctx, AssertionMode(mode), resourceType, name, "", expression)
		})

	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` matches expressions:$",
		func(ctx context.Context, mode string, resourceType string, name string, table *godog.Table) error {
			return ResourceMatchesExpressions(ctx, AssertionMode(mode), resourceType, name, "", table)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` to match expression `([^`]+)`",
		func(ctx context.Context, resourceType string, name string, expression string) error {
			return ResourceMatchesExpressions(ctx, ExpectMode, resourceType, name, "", expression)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` to match expression:$",
		func(ctx context.Context, resourceType string, name string, expression *godog.DocString) error {
			return ResourceMatchesExpressions(ctx, ExpectMode, resourceType, name, "", expression)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` to match expressions:$",
		func(ctx context.Context, resourceType string, name string, table *godog.Table) error {
			return ResourceMatchesExpressions(ctx, ExpectMode, resourceType, name, "", table)
		})

	// List matching with selectors
	ctx.Step(
		"^(eventually|consistently) `([^`]+)` with selector `([^`]+)` should match expression `([^`]+)`",
		func(ctx context.Context, mode string, resourceType string, selector string, expression string) error {
			return ResourceListMatchesExpressions(ctx, AssertionMode(mode), resourceType, "", selector, expression)
		})

	ctx.Step(
		"^(eventually|consistently) `([^`]+)` with selector `([^`]+)` should match expression:$",
		func(ctx context.Context, mode string, resourceType string, selector string, expression *godog.DocString) error {
			return ResourceListMatchesExpressions(ctx, AssertionMode(mode), resourceType, "", selector, expression)
		})

	ctx.Step(
		"^(eventually|consistently) `([^`]+)` with selector `([^`]+)` should match expressions:$",
		func(ctx context.Context, mode string, resourceType string, selector string, table *godog.Table) error {
			return ResourceListMatchesExpressions(ctx, AssertionMode(mode), resourceType, "", selector, table)
		})

	ctx.Step(
		"^expect `([^`]+)` with selector `([^`]+)` to match expression `([^`]+)`",
		func(ctx context.Context, resourceType string, selector string, expression string) error {
			return ResourceListMatchesExpressions(ctx, ExpectMode, resourceType, "", selector, expression)
		})

	ctx.Step(
		"^expect `([^`]+)` with selector `([^`]+)` to match expression:$",
		func(ctx context.Context, resourceType string, selector string, expression *godog.DocString) error {
			return ResourceListMatchesExpressions(ctx, ExpectMode, resourceType, "", selector, expression)
		})

	ctx.Step(
		"^expect `([^`]+)` with selector `([^`]+)` to match expressions:$",
		func(ctx context.Context, resourceType string, selector string, table *godog.Table) error {
			return ResourceListMatchesExpressions(ctx, ExpectMode, resourceType, "", selector, table)
		})

	// Resource deletion
	ctx.Step(
		"^I delete the `([^`]+)` `([^`]+)`",
		func(ctx context.Context, resourceType string, name string) error {
			return DeleteResource(ctx, resourceType, name, "")
		})

	// Resource updates
	ctx.Step(
		"^I update the `([^`]+)` `([^`]+)` with expression `([^`]+)`",
		func(ctx context.Context, resourceType string, name string, expression string) error {
			return UpdateResourceWithExpressions(ctx, resourceType, name, "", expression)
		})

	ctx.Step(
		"^I update the `([^`]+)` `([^`]+)` with expression:$",
		func(ctx context.Context, resourceType string, name string, expression *godog.DocString) error {
			return UpdateResourceWithExpressions(ctx, resourceType, name, "", expression)
		})

	ctx.Step(
		"^I update the `([^`]+)` `([^`]+)` with expressions:$",
		func(ctx context.Context, resourceType string, name string, table *godog.Table) error {
			return UpdateResourceWithExpressions(ctx, resourceType, name, "", table)
		})

	// Conditions
	ctx.Step(
		"^(eventually|consistently) the `([^`]+)` `([^`]+)` has conditions:$",
		func(ctx context.Context, mode string, resourceType string, name string, table *godog.Table) error {
			return ResourceHasConditions(ctx, AssertionMode(mode), resourceType, name, "", table)
		})

	ctx.Step(
		"^expect the `([^`]+)` `([^`]+)` to have conditions:$",
		func(ctx context.Context, resourceType string, name string, table *godog.Table) error {
			return ResourceHasConditions(ctx, ExpectMode, resourceType, name, "", table)
		})
}

func initializeResourceTypeSteps(ctx *godog.ScenarioContext) {
	ctx.Step(
		"^(eventually|consistently) the resource type `([^`]+)` should exist",
		func(ctx context.Context, mode string, resourceType string) error {
			return ResourceType(ctx, AssertionMode(mode), resourceType, gomega.Not(gomega.BeNil()))
		})

	ctx.Step(
		"^(eventually|consistently) the resource type `([^`]+)` should not exist",
		func(ctx context.Context, mode string, resourceType string) error {
			return ResourceType(ctx, AssertionMode(mode), resourceType, gomega.BeNil())
		})

	ctx.Step(
		"^expect the resource type `([^`]+)` to exist",
		func(ctx context.Context, resourceType string) error {
			return ResourceType(ctx, ExpectMode, resourceType, gomega.Not(gomega.BeNil()))
		})

	ctx.Step(
		"^expect the resource type `([^`]+)` to not exist",
		func(ctx context.Context, resourceType string) error {
			return ResourceType(ctx, ExpectMode, resourceType, gomega.BeNil())
		})
}

func Resource(
	ctx context.Context,
	mode AssertionMode,
	resourceType string,
	name string,
	namespace string,
	matcher gtypes.GomegaMatcher,
) error {
	tc := TestCtx(ctx)

	nn := types.NamespacedName{
		Name:      tc.Variables().MustInterpolate(name),
		Namespace: tc.Variables().MustInterpolate(namespace),
	}

	c := CapturingT{}
	g := c.NewWithT(tc.config)

	Assert(ctx, g, mode, func(ctx context.Context) (any, error) {
		u, err := tc.resourceClient.Get(ctx, nn, resourceType)
		if err != nil {
			return nil, client.IgnoreNotFound(err)
		}

		return u, nil
	}).Should(
		matcher,
	)

	if err := c.Error(); err != nil {
		return err
	}

	return nil
}

func ResourceMatchesExpressions(
	ctx context.Context,
	mode AssertionMode,
	resourceType string,
	name string,
	namespace string,
	in any,
) error {
	tc := TestCtx(ctx)

	nn := types.NamespacedName{
		Name:      tc.Variables().MustInterpolate(name),
		Namespace: tc.Variables().MustInterpolate(namespace),
	}

	// Convert input to []string based on type
	expressions, err := ConvertToExpressions(in)
	if err != nil {
		return err
	}

	vars := tc.Variables().GetAll()

	for i := range expressions {
		expression, err := tc.Variables().Interpolate(expressions[i])
		if err != nil {
			return err
		}

		c := CapturingT{}
		g := c.NewWithT(tc.config)

		Assert(ctx, g, mode, func(ctx context.Context) (any, error) {
			u, err := tc.resourceClient.Get(ctx, nn, resourceType)
			if err != nil {
				return nil, err
			}

			return u, nil
		}).Should(
			jq.WithVariables(vars).Match(expression),
		)

		if err := c.Error(); err != nil {
			return err
		}
	}

	return nil
}

func ResourceListMatchesExpressions(
	ctx context.Context,
	mode AssertionMode,
	resourceType string,
	namespace string,
	selector string,
	in any,
) error {
	tc := TestCtx(ctx)

	interpolatedNamespace, err := tc.Variables().Interpolate(namespace)
	if err != nil {
		return err
	}

	interpolatedSelector, err := tc.Variables().Interpolate(selector)
	if err != nil {
		return err
	}

	labelSelector, err := labels.Parse(interpolatedSelector)
	if err != nil {
		return fmt.Errorf("invalid selector: %w", err)
	}

	// Convert input to []string based on type
	expressions, err := ConvertToExpressions(in)
	if err != nil {
		return err
	}

	vars := tc.Variables().GetAll()

	for _, expression := range expressions {
		interpolatedExpression, err := tc.Variables().Interpolate(expression)
		if err != nil {
			return err
		}

		c := CapturingT{}
		g := c.NewWithT(tc.config)
		m := jq.WithVariables(vars).Match(interpolatedExpression)

		Assert(ctx, g, mode, func(ctx context.Context) (*unstructured.UnstructuredList, error) {
			res, err := tc.resourceClient.List(ctx, interpolatedNamespace, resourceType, labelSelector)
			if err != nil {
				return nil, err
			}

			return res, nil
		}).Should(m)

		if err := c.Error(); err != nil {
			return fmt.Errorf("resource %s/%s with selector %s does not match expression %s: %w",
				interpolatedNamespace,
				resourceType,
				interpolatedSelector,
				interpolatedExpression, err)
		}
	}

	return nil
}

func UpdateResourceWithExpressions(
	ctx context.Context,
	resourceType string,
	name string,
	namespace string,
	in any,
) error {
	tc := TestCtx(ctx)

	nn := types.NamespacedName{
		Name:      tc.Variables().MustInterpolate(name),
		Namespace: tc.Variables().MustInterpolate(namespace),
	}

	// Convert input to []string based on type
	expressions, err := ConvertToExpressions(in)
	if err != nil {
		return err
	}

	c := CapturingT{}
	g := c.NewWithT(tc.config)

	g.Eventually(func(ctx context.Context) error {
		u, err := tc.resourceClient.Get(ctx, nn, resourceType)
		if err != nil {
			return err
		}

		obj, err := tc.JQ().Transform(u, expressions)
		if err != nil {
			return err
		}

		err = tc.Client().Update(ctx, obj)
		if err != nil {
			return err
		}

		return nil
	}).WithContext(ctx).Should(
		gomega.Succeed(),
	)

	if err := c.Error(); err != nil {
		return err
	}

	return nil
}

func ResourceType(
	ctx context.Context,
	mode AssertionMode,
	resourceType string,
	matcher gtypes.GomegaMatcher,
) error {
	tc := TestCtx(ctx)

	interpolatedResourceType, err := tc.Variables().Interpolate(resourceType)
	if err != nil {
		return err
	}

	c := CapturingT{}
	g := c.NewWithT(tc.config)

	Assert(ctx, g, mode, func(context.Context) (any, error) {
		resource, err := tc.resolver.Resolve(interpolatedResourceType)
		if err != nil {
			return nil, err
		}
		return resource, nil
	}).Should(matcher)

	if err := c.Error(); err != nil {
		return err
	}

	return nil
}

func ResourceHasConditions(
	ctx context.Context,
	mode AssertionMode,
	resourceType string,
	name string,
	namespace string,
	table *godog.Table,
) error {
	tc := TestCtx(ctx)

	nn := types.NamespacedName{
		Name:      tc.Variables().MustInterpolate(name),
		Namespace: tc.Variables().MustInterpolate(namespace),
	}

	conditionMatchers, err := TableToConditionMatchers(tc, table)
	if err != nil {
		return fmt.Errorf("failed to parse condition table: %w", err)
	}

	c := CapturingT{}
	g := c.NewWithT(tc.config)

	// For each condition type, check if it exists and matches
	Assert(ctx, g, mode, func(ctx context.Context) (any, error) {
		u, err := tc.resourceClient.Get(ctx, nn, resourceType)
		if err != nil {
			return nil, err
		}

		conditions, found, err := unstructured.NestedSlice(u.Object, "status", "conditions")
		if err != nil {
			return nil, fmt.Errorf("failed to get conditions from resource: %w", err)
		}
		if !found {
			return nil, errors.New("resource does not have status.conditions field")
		}

		// Transform conditions slice into a map keyed by type
		conditionsMap := make(map[string]interface{})
		for _, condition := range conditions {
			conditionMap, ok := condition.(map[string]interface{})
			if !ok {
				continue
			}
			conditionType, exists := conditionMap["type"]
			if !exists {
				continue
			}
			typeStr, ok := conditionType.(string)
			if !ok {
				continue
			}
			conditionsMap[typeStr] = conditionMap
		}

		return conditionsMap, nil
	}).Should(
		gstruct.MatchKeys(gstruct.IgnoreExtras, conditionMatchers),
	)

	if err := c.Error(); err != nil {
		return err
	}

	return nil
}

func DeleteResource(
	ctx context.Context,
	resourceType string,
	name string,
	namespace string,
) error {
	tc := TestCtx(ctx)

	nn := types.NamespacedName{
		Name:      tc.Variables().MustInterpolate(name),
		Namespace: tc.Variables().MustInterpolate(namespace),
	}

	if err := tc.resourceClient.Delete(ctx, nn, resourceType); err != nil {
		return client.IgnoreNotFound(err)
	}

	return nil
}

func CreateResourceFromYAML(ctx context.Context, content string) error {
	obj, err := Decode(ctx, content)
	if err != nil {
		return err
	}

	tc := TestCtx(ctx)
	if err := tc.Client().Create(ctx, obj); err != nil {
		return fmt.Errorf("failed to create resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}

func ApplyResourceFromYAML(ctx context.Context, content string) error {
	obj, err := Decode(ctx, content)
	if err != nil {
		return err
	}

	tc := TestCtx(ctx)
	if err := resources.Apply(ctx, tc.Client(), obj, client.ForceOwnership, client.FieldOwner(TestFieldOwner)); err != nil {
		return fmt.Errorf("failed to apply resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}

func CreateResourceFromTemplate(ctx context.Context, templateVar string, table *godog.Table) error {
	content, err := InterpolateFromVariable(ctx, templateVar, table)
	if err != nil {
		return err
	}
	obj, err := Decode(ctx, content)
	if err != nil {
		return err
	}

	tc := TestCtx(ctx)
	if err := tc.Client().Create(ctx, obj); err != nil {
		return fmt.Errorf("failed to create resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}

func ApplyResourceFromTemplate(ctx context.Context, templateVar string, table *godog.Table) error {
	content, err := InterpolateFromVariable(ctx, templateVar, table)
	if err != nil {
		return err
	}
	obj, err := Decode(ctx, content)
	if err != nil {
		return err
	}

	tc := TestCtx(ctx)
	if err := resources.Apply(ctx, tc.Client(), obj, client.ForceOwnership, client.FieldOwner(TestFieldOwner)); err != nil {
		return fmt.Errorf("failed to apply resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}
