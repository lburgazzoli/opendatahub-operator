/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package datasciencecluster contains controller logic of CRD DataScienceCluster
package datasciencecluster

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/gates"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/dependent"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
)

// Reconciler reconciles DataScienceCluster CRs.
type Reconciler struct {
	Options
}

// NewDataScienceClusterReconciler creates and registers the DSC reconciler.
// Registries default to the package-level singletons; override via options in tests.
func NewDataScienceClusterReconciler(ctx context.Context, mgr ctrl.Manager, opts ...Option) error {
	r := &Reconciler{
		Options: Options{
			ComponentRegistry: cr.DefaultRegistry(),
			ModuleRegistry:    modules.DefaultRegistry(),
			ProvisionRegistry: provision.DefaultRegistry(),
			DeletePropagation: metav1.DeletePropagationForeground,
		},
	}
	for _, opt := range opts {
		opt.applyOption(&r.Options)
	}

	componentsPredicate := dependent.New(dependent.WithWatchStatus(true))

	b := reconciler.ReconcilerFor(mgr, &dscv2.DataScienceCluster{}).
		WithDynamicOwnership().
		// Watch CRDs: when a module CRD is installed by the PlatformModule controller,
		// the Dynamic(CrdExists) guards on module OwnsGVK calls activate.
		WatchesGVK(
			gvk.CustomResourceDefinition,
			reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
			reconciler.WithEventMapper(func(ctx context.Context, _ client.Object) []reconcile.Request {
				return watchDataScienceClusters(ctx, mgr.GetClient())
			}),
		).
		// Static watches — DSC, GatewayConfig, ConfigMap.
		WatchesGVK(gvk.Tenant,
			reconciler.Dynamic(reconciler.CrdExists(gvk.Tenant)),
			reconciler.WithEventMapper(func(ctx context.Context, _ client.Object) []reconcile.Request {
				return watchDataScienceClusters(ctx, mgr.GetClient())
			}),
			reconciler.WithPredicates(componentsPredicate),
		).
		Watches(
			&dsciv2.DSCInitialization{},
			reconciler.WithEventMapper(func(ctx context.Context, _ client.Object) []reconcile.Request {
				return watchDataScienceClusters(ctx, mgr.GetClient())
			})).
		Watches(
			&serviceApi.GatewayConfig{},
			reconciler.WithEventMapper(func(ctx context.Context, _ client.Object) []reconcile.Request {
				return watchDataScienceClusters(ctx, mgr.GetClient())
			}),
			reconciler.WithPredicates(resources.GatewayConfigDomainChanged())).
		Watches(
			&corev1.ConfigMap{},
			reconciler.WithEventMapper(func(ctx context.Context, _ client.Object) []reconcile.Request {
				return watchDataScienceClusters(ctx, mgr.GetClient())
			}),
			reconciler.WithPredicates(
				resources.CreatedOrUpdatedOrDeletedNamed(gates.AcksConfigMap),
			))

	// Dynamic Owns for in-tree component CRs.
	_ = r.ComponentRegistry.ForEach(func(h cr.ComponentHandler) error {
		b = b.OwnsGVK(
			h.GetGroupVersionKind(),
			reconciler.Dynamic(reconciler.CrdExists(h.GetGroupVersionKind())),
			reconciler.WithPredicates(componentsPredicate),
		)

		return nil
	})

	_ = r.ModuleRegistry.ForAll(func(h modules.ModuleHandler, _ bool) error {
		switch h.GetGroupVersionKind() {
		// Monitoring is DSCI-owned — DSC must not handle it.
		case gvk.Monitoring:
			return nil
		default:
			b = b.OwnsGVK(
				h.GetGroupVersionKind(),
				reconciler.Dynamic(reconciler.CrdExists(h.GetGroupVersionKind())),
				reconciler.WithPredicates(componentsPredicate),
			)
		}

		return nil
	})

	_, err := b.
		WithAction(r.initialize).
		WithAction(r.checkPreConditions).
		WithAction(r.syncComponentDAGState).
		WithAction(r.updateStatus).
		WithAction(r.provisionComponents).
		WithAction(r.provisionModuleCRs).
		WithAction(r.syncPlatformModules).
		WithAction(deploy.NewAction(
			deploy.WithCache(),
			deploy.WithApplyOrder(),
			deploy.WithContinueOnError(),
		)).
		WithAction(r.cleanupDisabledComponents).
		WithAction(r.cleanupDisabledModules).
		WithConditions(status.ConditionTypeComponentsReady, status.ConditionTypeModulesReady).
		Build(ctx)

	if err != nil {
		return err
	}

	return nil
}
