package bdd

import (
	"context"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InitializeComponentSteps(ctx *godog.ScenarioContext) {
	initializeComponentManagementSteps(ctx)
}

func buildConditionExpression(componentName string, expectedStatus metav1.ConditionStatus) string {
	return fmt.Sprintf(`.status.conditions[] | select(.type | ascii_downcase == "%sready") | .status == "%s"`,
		strings.ToLower(componentName),
		expectedStatus,
	)
}

func buildComponentReadinessExpression(expectedStatus metav1.ConditionStatus) string {
	return fmt.Sprintf(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
		status.ConditionTypeReady,
		expectedStatus,
	)
}

func initializeComponentManagementSteps(ctx *godog.ScenarioContext) {
	ctx.Step(
		"^I set the `([^`]+)` component management state to `([^`]+)` in the DataScienceCluster$",
		func(ctx context.Context, componentName string, managementState string) error {
			tc := TestCtx(ctx)
			componentName = tc.Variables().MustInterpolate(componentName)

			expression := fmt.Sprintf(`.spec.components.%s.managementState = "%s"`, componentName, managementState)
			return UpdateResourceSingletonWithExpressions(ctx, "dsc", "", "", expression)
		})

	ctx.Step(
		"^(eventually|consistently) the component `([^`]+)` should be ready in the DataScienceCluster$",
		func(ctx context.Context, mode string, componentName string) error {
			tc := TestCtx(ctx)
			componentName = tc.Variables().MustInterpolate(componentName)

			expression := buildConditionExpression(componentName, metav1.ConditionTrue)
			return ResourceSingletonMatchesExpressions(ctx, AssertionMode(mode), "dsc", "", "", expression)
		})

	ctx.Step(
		"^(eventually|consistently) the component `([^`]+)` should not be ready in the DataScienceCluster$",
		func(ctx context.Context, mode string, componentName string) error {
			tc := TestCtx(ctx)
			componentName = tc.Variables().MustInterpolate(componentName)

			expression := buildConditionExpression(componentName, metav1.ConditionFalse)
			return ResourceSingletonMatchesExpressions(ctx, AssertionMode(mode), "dsc", "", "", expression)
		})

	ctx.Step(
		"^expect the component `([^`]+)` to be ready in the DataScienceCluster$",
		func(ctx context.Context, componentName string) error {
			tc := TestCtx(ctx)
			componentName = tc.Variables().MustInterpolate(componentName)

			expression := buildConditionExpression(componentName, metav1.ConditionTrue)
			return ResourceSingletonMatchesExpressions(ctx, ExpectMode, "dsc", "", "", expression)
		})

	ctx.Step(
		"^expect the component `([^`]+)` to not be ready in the DataScienceCluster$",
		func(ctx context.Context, componentName string) error {
			tc := TestCtx(ctx)
			componentName = tc.Variables().MustInterpolate(componentName)

			expression := buildConditionExpression(componentName, metav1.ConditionFalse)
			return ResourceSingletonMatchesExpressions(ctx, ExpectMode, "dsc", "", "", expression)
		})

	ctx.Step(
		"^(eventually|consistently) the component `([^`]+)` should be ready$",
		func(ctx context.Context, mode string, componentType string) error {
			tc := TestCtx(ctx)
			componentType = tc.Variables().MustInterpolate(componentType)

			expression := buildComponentReadinessExpression(metav1.ConditionTrue)
			return ResourceSingletonMatchesExpressions(ctx, AssertionMode(mode), componentType, "", "", expression)
		})

	ctx.Step(
		"^(eventually|consistently) the component `([^`]+)` should not be ready$",
		func(ctx context.Context, mode string, componentType string) error {
			tc := TestCtx(ctx)
			componentType = tc.Variables().MustInterpolate(componentType)

			expression := buildComponentReadinessExpression(metav1.ConditionFalse)
			return ResourceSingletonMatchesExpressions(ctx, AssertionMode(mode), componentType, "", "", expression)
		})

	ctx.Step(
		"^expect the component `([^`]+)` to be ready$",
		func(ctx context.Context, componentType string) error {
			tc := TestCtx(ctx)
			componentType = tc.Variables().MustInterpolate(componentType)

			expression := buildComponentReadinessExpression(metav1.ConditionTrue)
			return ResourceSingletonMatchesExpressions(ctx, ExpectMode, componentType, "", "", expression)
		})

	ctx.Step(
		"^expect the component `([^`]+)` to not be ready$",
		func(ctx context.Context, componentType string) error {
			tc := TestCtx(ctx)
			componentType = tc.Variables().MustInterpolate(componentType)

			expression := buildComponentReadinessExpression(metav1.ConditionFalse)
			return ResourceSingletonMatchesExpressions(ctx, ExpectMode, componentType, "", "", expression)
		})
}
