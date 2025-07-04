package bdd

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"k8s.io/apimachinery/pkg/types"
)

func InitializeConfigurationSteps(ctx *godog.ScenarioContext) {
	ctx.Step("^the eventually timeout is `([^`]+)`$", func(ctx context.Context, timeout string) error {
		tc := TestCtx(ctx)

		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout duration %s: %w", timeout, err)
		}
		tc.config.Eventually.Timeout = duration
		return nil
	})

	ctx.Step("^the eventually polling interval is `([^`]+)`$", func(ctx context.Context, interval string) error {
		tc := TestCtx(ctx)

		duration, err := time.ParseDuration(interval)
		if err != nil {
			return fmt.Errorf("invalid interval duration %s: %w", interval, err)
		}
		tc.config.Eventually.Interval = duration
		return nil
	})

	ctx.Step("^the consistently timeout is `([^`]+)`$", func(ctx context.Context, timeout string) error {
		tc := TestCtx(ctx)

		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout duration %s: %w", timeout, err)
		}
		tc.config.Consistently.Timeout = duration
		return nil
	})

	ctx.Step("^the consistently polling interval is `([^`]+)`$", func(ctx context.Context, interval string) error {
		tc := TestCtx(ctx)

		duration, err := time.ParseDuration(interval)
		if err != nil {
			return fmt.Errorf("invalid interval duration %s: %w", interval, err)
		}
		tc.config.Consistently.Interval = duration
		return nil
	})
}

func InitializeVariableSteps(ctx *godog.ScenarioContext) {
	ctx.Step("^I set variable `([^`]+)` to `([^`]+)`$", func(ctx context.Context, name, value string) error {
		tc := TestCtx(ctx)
		tc.Variables().Set(name, value)
		return nil
	})

	ctx.Step("^I set variable `([^`]+)` to:$", func(ctx context.Context, name string, value *godog.DocString) error {
		tc := TestCtx(ctx)
		tc.Variables().Set(name, value.Content)
		return nil
	})

	ctx.Step("^I set variable `([^`]+)` from `([^`]+)` `([^`]+)` using expression `([^`]+)`$",
		func(ctx context.Context, varName string, resourceType string, resourceName string, expression string) error {
			tc := TestCtx(ctx)

			resType, err := tc.Variables().Interpolate(resourceType)
			if err != nil {
				return err
			}
			resName, err := tc.Variables().Interpolate(resourceName)
			if err != nil {
				return err
			}

			nn := types.NamespacedName{
				Name: resName,
			}

			u, err := tc.resourceClient.Get(ctx, nn, resType)
			if err != nil {
				return fmt.Errorf("failed to get resource %s/%s: %w", resType, resName, err)
			}

			expr, err := tc.Variables().Interpolate(expression)
			if err != nil {
				return err
			}

			result, err := tc.JQ().Query(u, expr)
			if err != nil {
				return fmt.Errorf("failed to evaluate expression '%s': %w", expr, err)
			}

			tc.Variables().Set(varName, result)
			return nil
		})

	ctx.Step("^I set variable `([^`]+)` from `([^`]+)` `([^`]+)` in namespace `([^`]+)` using expression `([^`]+)`$",
		func(ctx context.Context, varName string, resourceType string, resourceName string, namespace string, expression string) error {
			tc := TestCtx(ctx)

			resType, err := tc.Variables().Interpolate(resourceType)
			if err != nil {
				return err
			}
			resNamespace, err := tc.Variables().Interpolate(namespace)
			if err != nil {
				return err
			}
			resName, err := tc.Variables().Interpolate(resourceName)
			if err != nil {
				return err
			}

			nn := types.NamespacedName{
				Name:      resName,
				Namespace: resNamespace,
			}

			u, err := tc.resourceClient.Get(ctx, nn, resType)
			if err != nil {
				return fmt.Errorf("failed to get resource %s/%s in namespace %s: %w", resType, resName, resNamespace, err)
			}

			expr, err := tc.Variables().Interpolate(expression)
			if err != nil {
				return err
			}

			result, err := tc.JQ().Query(u, expr)
			if err != nil {
				return fmt.Errorf("failed to evaluate expression '%s': %w", expr, err)
			}

			tc.Variables().Set(varName, result)

			return nil
		})

	ctx.Step("^I reset variable `([^`]+)`$", func(ctx context.Context, name string) error {
		tc := TestCtx(ctx)
		tc.Variables().Remove(name)

		return nil
	})
}
