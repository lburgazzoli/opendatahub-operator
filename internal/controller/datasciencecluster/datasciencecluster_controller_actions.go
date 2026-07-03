package datasciencecluster

import (
	"context"
	"fmt"
	"reflect"

	operatorv1 "github.com/openshift/api/operator/v1"
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
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
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

func initialize(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
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

func checkPreConditions(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	// This case should not happen, since there is a webhook that blocks the creation
	// of more than one instance of the DataScienceCluster, however one can create a
	// DataScienceCluster instance while the operator is stopped, hence this extra check

	if _, err := cluster.GetDSCI(ctx, rr.Client); err != nil {
		return fmt.Errorf("failed to get a valid DataScienceCluster instance, %w", err)
	}

	if _, err := cluster.GetDSC(ctx, rr.Client); err != nil {
		return fmt.Errorf("failed to get a valid DSCInitialization instance, %w", err)
	}

	return nil
}

func watchDataScienceClusters(ctx context.Context, cli client.Client) []reconcile.Request {
	return cluster.WatchDataScienceClusters(ctx, cli)
}

// provisionComponents creates CRs for all enabled in-tree components. The
// Platform controller now owns the DAG orchestration; DSC just creates the CRs
// directly without runlevel gating or readiness checking.
func provisionComponents(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	rr.Generated = true

	log := logf.FromContext(ctx)

	var failedComponents []string

	_ = cr.DefaultRegistry().ForEach(func(handler cr.ComponentHandler) error {
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
			Message: fmt.Sprintf("Provisioning failed for: %s", fmt.Sprintf("%v", failedComponents)),
		})

		return fmt.Errorf("provisioning failed for components: %v", failedComponents)
	}

	return nil
}

// provisionModuleCRs creates module operand CRs (e.g. AIGateway) for all
// enabled modules. It does NOT deploy module operator manifests — that is
// handled by the PlatformModule controller. Module CRD may not exist yet if
// the module operator hasn't been deployed; deploy.WithContinueOnError() handles
// the transient failure gracefully.
func provisionModuleCRs(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	reg := modules.DefaultRegistry()
	if !reg.HasEntries() {
		return nil
	}

	dsci, err := cluster.GetDSCI(ctx, rr.Client)
	if err != nil {
		return fmt.Errorf("failed to get DSCI for module provisioning: %w", err)
	}

	platformCtx := &modules.PlatformContext{
		DSC:  instance,
		DSCI: dsci,
	}
	platformCtx.ApplicationsNamespace, err = cluster.ApplicationNamespace(ctx, rr.Client)
	if err != nil {
		return fmt.Errorf("failed to resolve application namespace: %w", err)
	}

	log := logf.FromContext(ctx)
	var failedModules []string

	_ = reg.ForAll(func(handler modules.ModuleHandler, _ bool) error {
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

// syncPlatformModules SSA-patches Platform.Spec.Modules with the modules that
// DSC controls (currently AIGateway). The Platform controller reads this field
// to create PlatformModule CRs, which in turn deploy the module operators.
// DSC uses its own field manager so DSCI can independently own other module fields
// (e.g. Monitoring) via SSA without conflict.
func syncPlatformModules(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
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

	aiGatewayState := operatorv1.Removed
	if instance.Spec.Components.AIGateway.ManagementState == operatorv1.Managed {
		aiGatewayState = operatorv1.Managed
	}

	platform.Spec.Modules.AIGateway = common.ManagementSpec{ManagementState: aiGatewayState}

	if err := resources.Apply(ctx, rr.Client, platform,
		client.FieldOwner("datasciencecluster"),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("failed to patch Platform modules from DSC: %w", err)
	}

	return nil
}

func updateStatus(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return fmt.Errorf("resource instance %v is not a dscv2.DataScienceCluster)", rr.Instance)
	}

	instance.Status.Release = rr.Release

	if err := computeComponentsStatus(ctx, rr, cr.DefaultRegistry()); err != nil {
		return err
	}

	if cr.HasEntries() {
		if err := modules.ComputeModulesStatus(ctx, rr); err != nil {
			return err
		}
	}

	return nil
}
