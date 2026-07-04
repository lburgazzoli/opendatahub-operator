package datasciencecluster

import (
	"context"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	odhtype "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

const (
	// TODO: remove after https://issues.redhat.com/browse/RHOAIENG-15920
	finalizerName = "datasciencecluster.opendatahub.io/finalizer"
)

// persistAPI is implemented by component CRs that expose an alternative object
// for the deploy action to persist (e.g. when the "public" CR wraps an inner
// object that should actually be applied to the cluster).
type persistAPI interface {
	APIPersistObject() client.Object
}

func isNilInterface(v any) bool {
	return v == nil || (reflect.ValueOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}

func watchDataScienceClusters(ctx context.Context, cli client.Client) []reconcile.Request {
	return cluster.WatchDataScienceClusters(ctx, cli)
}

func (r *Reconciler) initialize(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	// TODO: remove after https://issues.redhat.com/browse/RHOAIENG-15920
	if controllerutil.RemoveFinalizer(instance, finalizerName) {
		if err := rr.Client.Update(ctx, instance); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) checkPreConditions(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	if _, err := cluster.GetDSCI(ctx, rr.Client); err != nil {
		return fmt.Errorf("failed to get a valid DSCInitialization instance, %w", err)
	}

	if _, err := cluster.GetDSC(ctx, rr.Client); err != nil {
		return fmt.Errorf("failed to get a valid DataScienceCluster instance, %w", err)
	}

	return nil
}

// cleanupDisabledComponents deletes component CRs for disabled in-tree components.
// Single-phase: components have no finalizer-based operator keepalive pattern.
func (r *Reconciler) cleanupDisabledComponents(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	log := logf.FromContext(ctx)

	_ = r.ComponentRegistry.ForEach(func(h cr.ComponentHandler) error {
		if h.IsEnabled(instance) {
			return nil
		}

		ci, err := h.NewCRObject(ctx, rr.Client, instance)
		if err != nil {
			return nil //nolint:nilerr
		}
		if isNilInterface(ci) {
			return nil
		}

		obj, ok := ci.(client.Object)
		if !ok {
			return nil
		}

		if err := rr.Client.Delete(ctx, obj, client.PropagationPolicy(r.DeletePropagation)); client.IgnoreNotFound(err) != nil {
			log.Error(err, "failed to delete disabled component CR", "component", h.GetName())
		}

		return nil
	})

	return nil
}

// cleanupDisabledModules deletes module operand CRs for disabled modules.
// handler.DeleteModuleCR() is idempotent and handles NotFound/IsNoMatchError.
// DSCI is intentionally excluded from the PlatformContext: DSCI-owned modules
// (e.g. Monitoring) must not be deleted by the DSC controller. Each handler's
// IsEnabled reads only ctx.DSC, so DSCI-owned handlers return false and are
// skipped by their own controller's cleanup path.
func (r *Reconciler) cleanupDisabledModules(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	if !r.ModuleRegistry.HasEntries() {
		return nil
	}

	platformCtx := &modules.PlatformContext{DSC: instance}

	_ = r.ModuleRegistry.ForAll(func(h modules.ModuleHandler, _ bool) error {
		if h.IsEnabled(platformCtx) {
			return nil
		}
		// DeleteModuleCR handles NotFound and IsNoMatchError internally.
		_ = h.DeleteModuleCR(ctx, rr.Client)
		return nil
	})

	return nil
}

// provisionComponents creates CRs for all enabled in-tree components.
// The Platform controller owns DAG orchestration; DSC creates CRs directly.
func (r *Reconciler) provisionComponents(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	rr.Generated = true

	log := logf.FromContext(ctx)

	var failedComponents []string

	_ = r.ComponentRegistry.ForEach(func(handler cr.ComponentHandler) error {
		if !handler.IsEnabled(instance) {
			return nil
		}

		name := handler.GetName()

		ci, err := handler.NewCRObject(ctx, rr.Client, instance)
		if err != nil {
			log.Error(err, "NewCRObject failed", "component", name)
			failedComponents = append(failedComponents, name)
			return nil
		}
		if isNilInterface(ci) {
			return nil
		}
		obj, ok := ci.(client.Object)
		if !ok {
			log.Error(nil, "component CR does not implement client.Object",
				"component", name, "type", fmt.Sprintf("%T", ci))
			failedComponents = append(failedComponents, name)
			return nil
		}
		if p, ok := ci.(persistAPI); ok {
			if inner := p.APIPersistObject(); !isNilInterface(inner) {
				obj = inner
			}
		}
		if err := rr.AddResources(obj); err != nil {
			log.Error(err, "AddResources failed", "component", name)
			failedComponents = append(failedComponents, name)
		}

		return nil
	})

	if len(failedComponents) > 0 {
		rr.Conditions.SetCondition(common.Condition{
			Type:    status.ConditionTypeComponentsReady,
			Status:  metav1.ConditionFalse,
			Reason:  status.ProvisioningFailedReason,
			Message: fmt.Sprintf("Provisioning failed for: %v", failedComponents),
		})

		return fmt.Errorf("provisioning failed for components: %v", failedComponents)
	}

	return nil
}

// provisionModuleCRs creates module operand CRs (e.g. AIGateway) for enabled modules.
// Does NOT deploy module operator manifests — that is the PlatformModule controller's job.
func (r *Reconciler) provisionModuleCRs(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	if !r.ModuleRegistry.HasEntries() {
		return nil
	}

	// DSCI is intentionally excluded from the PlatformContext: DSCI-owned modules
	// (e.g. Monitoring) must not be provisioned by the DSC controller. Each
	// handler's IsEnabled reads only ctx.DSC, so DSCI-owned handlers return false.
	platformCtx := &modules.PlatformContext{DSC: instance}

	var err error
	platformCtx.ApplicationsNamespace, err = cluster.ApplicationNamespace(ctx, rr.Client)
	if err != nil {
		return fmt.Errorf("failed to resolve application namespace: %w", err)
	}

	log := logf.FromContext(ctx)
	var failedModules []string

	_ = r.ModuleRegistry.ForAll(func(handler modules.ModuleHandler, _ bool) error {
		if !handler.IsEnabled(platformCtx) {
			return nil
		}

		name := handler.GetName()

		moduleCR, err := handler.BuildModuleCR(ctx, rr.Client, platformCtx)
		if err != nil {
			log.Error(err, "BuildModuleCR failed", "module", name)
			failedModules = append(failedModules, name)
			return nil
		}
		if moduleCR == nil {
			log.Error(nil, "BuildModuleCR returned nil without error", "module", name)
			failedModules = append(failedModules, name)
			return nil
		}

		rr.Resources = append(rr.Resources, *moduleCR)
		return nil
	})

	if len(failedModules) > 0 {
		return fmt.Errorf("BuildModuleCR failed for modules: %v", failedModules)
	}

	return nil
}

// syncPlatformModules adds a Platform CR patch to rr.Resources so the deploy
// action SSA-applies Platform.Spec.Modules with the modules DSC controls.
// Each module handler writes its own field via ApplyManagementState, so this
// action does not need updating when new modules are onboarded.
func (r *Reconciler) syncPlatformModules(_ context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	platform := &configv1alpha1.Platform{}
	platform.Name = configv1alpha1.PlatformInstanceName
	platform.TypeMeta = metav1.TypeMeta{
		APIVersion: configv1alpha1.GroupVersion.String(),
		Kind:       configv1alpha1.PlatformKind,
	}

	platformCtx := &modules.PlatformContext{DSC: instance}
	_ = r.ModuleRegistry.ForAll(func(h modules.ModuleHandler, _ bool) error {
		h.ApplyManagementState(platformCtx, &platform.Spec.Modules)
		return nil
	})

	return rr.AddResources(platform)
}

func (r *Reconciler) updateStatus(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	instance.Status.Release = rr.Release

	if err := computeComponentsStatus(ctx, rr, r.ComponentRegistry); err != nil {
		return err
	}

	if err := modules.ComputeModulesStatus(ctx, rr, r.ModuleRegistry); err != nil {
		return err
	}

	return nil
}
