//nolint:maintidx
package cleanup_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	gTypes "github.com/onsi/gomega/types"
	"github.com/operator-framework/api/pkg/lib/version"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/xid"
	"github.com/stretchr/testify/mock"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	ctrlCli "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/cleanup"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"

	. "github.com/onsi/gomega"
)

var (
	v001 = semver.Version{Major: 0, Minor: 0, Patch: 1}
	v010 = semver.Version{Major: 0, Minor: 1, Patch: 0}

	successOrNotFound = Or(
		Not(HaveOccurred()),
		MatchError(
			k8serr.IsNotFound,
			"IsNotFound",
		),
	)

	expectNotFound = MatchError(
		k8serr.IsNotFound, "IsNotFound",
	)
)

//nolint:gochecknoinits
func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

// Helper function to create a test CRD that should never be deleted (protected type).
func createTestCRD(id string) apiextensionsv1.CustomResourceDefinition {
	return apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foos." + id + ".opendatahub.io",
			Labels: map[string]string{
				labels.PlatformPartOf: labels.Platform,
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: id + ".opendatahub.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     "Foo",
				ListKind: "FooList",
				Plural:   "foos",
				Singular: "foo",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}
}

func uuidFromInstance(rr *types.ReconciliationRequest) string {
	return string(rr.Instance.GetUID())
}

func NewTestNamespace() corev1.Namespace {
	return corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: xid.New().String(),
		},
	}
}

type ResourceOption func(obj ctrlCli.Object, rr *types.ReconciliationRequest) error

func OwnedBy(owner ctrlCli.Object) ResourceOption {
	return func(obj ctrlCli.Object, rr *types.ReconciliationRequest) error {
		return controllerutil.SetOwnerReference(owner, obj, rr.Client.Scheme())
	}
}

func CreateTestResource[T ctrlCli.Object](
	t *testing.T,
	rr *types.ReconciliationRequest,
	obj T,
	opts ...ResourceOption,
) T {
	t.Helper()

	g := NewWithT(t)
	ctx := t.Context()

	// Apply all options
	for _, opt := range opts {
		g.Expect(opt(obj, rr)).NotTo(HaveOccurred())
	}

	// Create the resource
	g.Expect(rr.Client.Create(ctx, obj)).NotTo(HaveOccurred())

	// Register cleanup
	t.Cleanup(func() {
		// Use background context for cleanup since test context may be cancelled
		//nolint:usetesting
		cleanupCtx := context.Background()

		g.Expect(rr.Client.Delete(cleanupCtx, obj)).Should(successOrNotFound)
	})

	return obj
}

type ReconciliationRequestOption func(rr *types.ReconciliationRequest)

func WithVersion(value semver.Version) ReconciliationRequestOption {
	return func(rr *types.ReconciliationRequest) {
		rr.Release.Version.Version = value
	}
}

func WithGenerated(value bool) ReconciliationRequestOption {
	return func(rr *types.ReconciliationRequest) {
		rr.Generated = value
	}
}

func WithInstance(value common.PlatformObject) ReconciliationRequestOption {
	return func(rr *types.ReconciliationRequest) {
		rr.Instance = value
		rr.Kind = value.GetObjectKind().GroupVersionKind()
	}
}

func NewReconciliationRequest(envTest *envt.EnvT, opts ...ReconciliationRequestOption) types.ReconciliationRequest {
	rr := types.ReconciliationRequest{
		Client: envTest.Client(),
		DSCI: &dsciv1.DSCInitialization{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
		},
		Kind: gvk.Dashboard,
		Instance: &componentApi.Dashboard{
			TypeMeta: metav1.TypeMeta{
				APIVersion: componentApi.GroupVersion.String(),
				Kind:       componentApi.DashboardKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: componentApi.DashboardInstanceName,
			},
		},
		Release: common.Release{
			Name: cluster.OpenDataHub,
			Version: version.OperatorVersion{
				Version: v001,
			},
		},
		Generated: true,
		Controller: mocks.NewMockController(func(m *mocks.MockController) {
			m.On("GetClient").Return(envTest.Client())
			m.On("GetDynamicClient").Return(envTest.DynamicClient())
			m.On("GetDiscoveryClient").Return(envTest.DiscoveryClient())
			m.On("Owns", mock.Anything).Return(false)
		}),
	}

	for _, opt := range opts {
		opt(&rr)
	}

	return rr
}

//nolint:maintidx
func TestCleanupAction_Delete(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		version               semver.Version
		generated             bool
		matcher               gTypes.GomegaMatcher
		cyclesMetricsMatcher  gTypes.GomegaMatcher
		deletedMetricsMatcher gTypes.GomegaMatcher
		deOwnedMetricsMatcher gTypes.GomegaMatcher
		labels                map[string]string
		annotations           map[string]string
		options               []cleanup.ActionOption
		uidFn                 func(request *types.ReconciliationRequest) string
	}{
		{
			name:                  "should delete leftovers",
			version:               v001,
			generated:             true,
			matcher:               Satisfy(k8serr.IsNotFound),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 2),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not delete resources because same annotations",
			version:               v010,
			generated:             true,
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not delete resources because unmanaged",
			version:               v010,
			generated:             true,
			annotations:           map[string]string{annotations.ManagedByODHOperator: "false"},
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not delete resources because of no generated resources have been detected",
			version:               v001,
			generated:             false,
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 0),
			deletedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not delete resources because of selector",
			version:               v001,
			generated:             true,
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			labels:                map[string]string{"foo": "bar"},
			options:               []cleanup.ActionOption{cleanup.WithLabels(map[string]string{"foo": "baz"})},
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not delete resources because of unremovable type",
			version:               v001,
			generated:             true,
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			options:               []cleanup.ActionOption{cleanup.WithProtectedTypes(gvk.ConfigMap, gvk.ClusterRole)},
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should delete leftovers because of UID",
			version:               v010,
			generated:             true,
			matcher:               Satisfy(k8serr.IsNotFound),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 2),
			uidFn:                 func(rr *types.ReconciliationRequest) string { return xid.New().String() },
		},
		{
			name:                  "should not delete leftovers because of UID",
			version:               v010,
			generated:             true,
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeownedTotal.Reset()
			cleanup.DeownedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			id := xid.New().String()

			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest, WithVersion(tt.version), WithGenerated(tt.generated))

			CreateTestResource(t, &rr, &ns)

			CreateTestResource(t, &rr, rr.Instance)

			// should never get deleted
			crd := createTestCRD(id)

			t.Cleanup(func() {
				g.Eventually(func() error {
					return cli.Delete(ctx, &crd)
				}).Should(successOrNotFound)
			})

			g.Expect(cli.Create(ctx, &crd)).
				NotTo(HaveOccurred())

			om := metav1.ObjectMeta{
				Annotations: map[string]string{
					annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
					annotations.InstanceUID:        tt.uidFn(&rr),
					annotations.PlatformVersion:    v010.String(),
					annotations.PlatformType:       string(cluster.OpenDataHub),
				},
				Labels: map[string]string{
					labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
				},
			}

			for k, v := range tt.labels {
				om.Labels[k] = v
			}
			for k, v := range tt.annotations {
				om.Annotations[k] = v
			}

			// should never get deleted
			l := coordinationv1.Lease{ObjectMeta: *om.DeepCopy()}
			l.Name = id
			l.Namespace = ns.Name

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &l)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &l, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &l)).
				NotTo(HaveOccurred())

			cm := corev1.ConfigMap{ObjectMeta: *om.DeepCopy()}
			cm.Name = "gc-cm"
			cm.Namespace = ns.Name

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			cr := rbacv1.ClusterRole{ObjectMeta: *om.DeepCopy()}
			cr.Name = "gc-cr"

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			opts := make([]cleanup.ActionOption, 0, len(tt.options)+1)
			opts = append(opts, cleanup.InNamespace(ns.Name))
			opts = append(opts, cleanup.WithHandler(cleanup.DefaultDeleteHandler(metav1.DeletePropagationBackground)))
			opts = append(opts, tt.options...)

			a := cleanup.NewAction(opts...)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&l), &coordinationv1.Lease{})
			g.Expect(err).ToNot(HaveOccurred())

			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&crd), &apiextensionsv1.CustomResourceDefinition{})
			g.Expect(err).ToNot(HaveOccurred())

			if tt.matcher != nil {
				g.Eventually(func() error {
					return cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
				}).Should(
					tt.matcher,
				)
				g.Eventually(func() error {
					return cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cr), &rbacv1.ClusterRole{})
				}).Should(
					tt.matcher,
				)
			}

			if tt.cyclesMetricsMatcher != nil {
				g.Expect(cleanup.CyclesTotal).Should(WithTransform(testutil.ToFloat64, tt.cyclesMetricsMatcher))
			}
			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
			if tt.deOwnedMetricsMatcher != nil {
				g.Expect(cleanup.DeownedTotal).Should(WithTransform(testutil.ToFloat64, tt.deOwnedMetricsMatcher))
			}

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
			if tt.deOwnedMetricsMatcher != nil {
				g.Expect(cleanup.DeownedTotal).Should(WithTransform(testutil.ToFloat64, tt.deOwnedMetricsMatcher))
			}
		})
	}
}

func TestCleanupAction_DeleteOwn(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name    string
		matcher gTypes.GomegaMatcher
		options cleanup.DeleteHandlerOptions
		owned   bool
	}{
		{
			name:    "should delete owned resources",
			matcher: Satisfy(k8serr.IsNotFound),
			owned:   true,
		},
		{
			name:    "should not delete non owned resources",
			matcher: Not(HaveOccurred()),
			owned:   false,
		},
		{
			name:    "should delete non owned resources",
			matcher: Satisfy(k8serr.IsNotFound),
			owned:   false,
			options: cleanup.DeleteHandlerOptions{
				CheckOwnership: ptr.To(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)

			CreateTestResource(t, &rr, rr.Instance)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cleanup-own-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			if tt.owned {
				CreateTestResource(t, &rr, cm, OwnedBy(rr.Instance))
			} else {
				CreateTestResource(t, &rr, cm)
			}

			// force background as there's no controller
			tt.options.PropagationPolicy = metav1.DeletePropagationBackground

			a := cleanup.NewAction(
				cleanup.WithHandler(cleanup.DeleteHandler(&tt.options)),
				cleanup.InNamespace(ns.Name),
			)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.matcher != nil {
				g.Eventually(func() error {
					return cli.Get(ctx, ctrlCli.ObjectKeyFromObject(cm), &corev1.ConfigMap{})
				}).Should(
					tt.matcher,
				)
			}
		})
	}
}

//nolint:maintidx
func TestCleanupAction_DeOwn(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		version               semver.Version
		generated             bool
		ownerRefMatcher       gTypes.GomegaMatcher
		cyclesMetricsMatcher  gTypes.GomegaMatcher
		deletedMetricsMatcher gTypes.GomegaMatcher
		deOwnedMetricsMatcher gTypes.GomegaMatcher
		labels                map[string]string
		annotations           map[string]string
		options               []cleanup.ActionOption
		uidFn                 func(request *types.ReconciliationRequest) string
	}{
		{
			name:                  "should remove owner references",
			version:               v001,
			generated:             true,
			ownerRefMatcher:       HaveLen(0), // Should have no owner references after cleanup
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0), // No deletions, only deowning
			deOwnedMetricsMatcher: BeNumerically("==", 2),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not remove owner references because same annotations",
			version:               v010,
			generated:             true,
			ownerRefMatcher:       HaveLen(1), // Should still have owner reference
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deOwnedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not remove owner references because unmanaged",
			version:               v010,
			generated:             true,
			annotations:           map[string]string{annotations.ManagedByODHOperator: "false"},
			ownerRefMatcher:       HaveLen(1), // Should still have owner reference
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deOwnedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not remove owner references because no generated resources detected",
			version:               v001,
			generated:             false,
			ownerRefMatcher:       HaveLen(1), // Should still have owner reference
			cyclesMetricsMatcher:  BeNumerically("==", 0),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deOwnedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should remove owner references because of UID mismatch",
			version:               v010,
			generated:             true,
			ownerRefMatcher:       HaveLen(0), // Should have no owner references after cleanup
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deOwnedMetricsMatcher: BeNumerically("==", 2),
			uidFn:                 func(rr *types.ReconciliationRequest) string { return xid.New().String() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeownedTotal.Reset()
			cleanup.DeownedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			id := xid.New().String()

			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest, WithVersion(tt.version), WithGenerated(tt.generated))

			CreateTestResource(t, &rr, &ns)

			CreateTestResource(t, &rr, rr.Instance)

			// should never get deowned (protected type)
			crd := createTestCRD(id)

			t.Cleanup(func() {
				g.Eventually(func() error {
					return cli.Delete(ctx, &crd)
				}).Should(successOrNotFound)
			})

			g.Expect(cli.Create(ctx, &crd)).
				NotTo(HaveOccurred())

			om := metav1.ObjectMeta{
				Annotations: map[string]string{
					annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
					annotations.InstanceUID:        tt.uidFn(&rr),
					annotations.PlatformVersion:    v010.String(),
					annotations.PlatformType:       string(cluster.OpenDataHub),
				},
				Labels: map[string]string{
					labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
				},
			}

			for k, v := range tt.labels {
				om.Labels[k] = v
			}
			for k, v := range tt.annotations {
				om.Annotations[k] = v
			}

			// should never get deowned (protected type)
			l := coordinationv1.Lease{ObjectMeta: *om.DeepCopy()}
			l.Name = id
			l.Namespace = ns.Name

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &l)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &l, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &l)).
				NotTo(HaveOccurred())

			cm := corev1.ConfigMap{ObjectMeta: *om.DeepCopy()}
			cm.Name = "deown-cm"
			cm.Namespace = ns.Name

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			cr := rbacv1.ClusterRole{ObjectMeta: *om.DeepCopy()}
			cr.Name = "deown-cr"

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			opts := make([]cleanup.ActionOption, 0, len(tt.options)+2)
			opts = append(opts, cleanup.InNamespace(ns.Name))
			opts = append(opts, cleanup.WithHandler(cleanup.DeOwnHandler(
				cleanup.WithManagedAnnotationCheck(true),
			)))
			opts = append(opts, tt.options...)

			a := cleanup.NewAction(opts...)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			// Verify protected types (CRD and Lease) still have their owner references
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&l), &coordinationv1.Lease{})
			g.Expect(err).ToNot(HaveOccurred())

			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&crd), &apiextensionsv1.CustomResourceDefinition{})
			g.Expect(err).ToNot(HaveOccurred())

			if tt.ownerRefMatcher != nil {
				g.Eventually(func() ([]metav1.OwnerReference, error) {
					// Check ConfigMap still exists and verify owner references state
					res := corev1.ConfigMap{}
					if err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &res); err != nil {
						return nil, err
					}

					return res.OwnerReferences, nil
				}).Should(
					tt.ownerRefMatcher,
				)

				g.Eventually(func() ([]metav1.OwnerReference, error) {
					// Check ConfigMap still exists and verify owner references state
					res := rbacv1.ClusterRole{}
					if err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cr), &res); err != nil {
						return nil, err
					}

					return res.OwnerReferences, nil
				}).Should(
					tt.ownerRefMatcher,
				)
			}

			// Check metrics
			if tt.cyclesMetricsMatcher != nil {
				g.Expect(cleanup.CyclesTotal).Should(WithTransform(testutil.ToFloat64, tt.cyclesMetricsMatcher))
			}
			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
			if tt.deOwnedMetricsMatcher != nil {
				g.Expect(cleanup.DeownedTotal).Should(WithTransform(testutil.ToFloat64, tt.deOwnedMetricsMatcher))
			}

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
			if tt.deOwnedMetricsMatcher != nil {
				g.Expect(cleanup.DeownedTotal).Should(WithTransform(testutil.ToFloat64, tt.deOwnedMetricsMatcher))
			}
		})
	}
}

func TestCleanupAction_CustomHandler(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		cmMatcher             gTypes.GomegaMatcher
		crOwnerRefMatcher     gTypes.GomegaMatcher
		deletedMetricsMatcher gTypes.GomegaMatcher
		deOwnedMetricsMatcher gTypes.GomegaMatcher
		options               cleanup.DeleteHandlerOptions
		owned                 bool
	}{
		{
			name:                  "should delete ConfigMaps and deown ClusterRoles when owned",
			cmMatcher:             Satisfy(k8serr.IsNotFound), // ConfigMap should be deleted
			crOwnerRefMatcher:     HaveLen(0),                 // ClusterRole should have no owner refs
			deletedMetricsMatcher: BeNumerically("==", 1),     // 1 ConfigMap deleted
			deOwnedMetricsMatcher: BeNumerically("==", 1),     // 1 ClusterRole deowned
			owned:                 true,
		},
		{
			name:                  "should not process non-owned resources",
			cmMatcher:             Not(HaveOccurred()),    // ConfigMap should exist
			crOwnerRefMatcher:     HaveLen(0),             // ClusterRole has no owner refs anyway
			deletedMetricsMatcher: BeNumerically("==", 0), // No deletions
			deOwnedMetricsMatcher: BeNumerically("==", 0), // No deowning
			owned:                 false,
		},
		{
			name:                  "should process all resources when configured",
			cmMatcher:             Satisfy(k8serr.IsNotFound), // ConfigMap should be deleted
			crOwnerRefMatcher:     HaveLen(0),                 // ClusterRole should have no owner refs
			deletedMetricsMatcher: BeNumerically("==", 1),     // 1 ConfigMap deleted
			deOwnedMetricsMatcher: BeNumerically("==", 0),     // 1 ClusterRole had no owner already
			owned:                 false,
			options: cleanup.DeleteHandlerOptions{
				CheckOwnership: ptr.To(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeownedTotal.Reset()
			cleanup.DeownedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)

			CreateTestResource(t, &rr, rr.Instance)

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-handler-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			if tt.owned {
				g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
					NotTo(HaveOccurred())
			}

			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			cr := rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "custom-handler-cr",
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(successOrNotFound)
			})

			if tt.owned {
				g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
					NotTo(HaveOccurred())
			}

			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			// force background as there's no controller
			tt.options.PropagationPolicy = metav1.DeletePropagationBackground

			// Configure handlers based on test options
			deleteHandler := cleanup.DeleteHandler(&tt.options)
			deownHandler := cleanup.DeOwnHandler()

			customHandler := func(ctx context.Context, rr *types.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
				switch obj.GroupVersionKind() {
				case gvk.ConfigMap:
					// Delete ConfigMaps
					return deleteHandler(ctx, rr, obj)
				case gvk.ClusterRole:
					// Deown ClusterRoles by removing Dashboard owner references
					return deownHandler(ctx, rr, obj)
				default:
					// For other types, do nothing
					return false, nil
				}
			}

			a := cleanup.NewAction(cleanup.WithHandler(customHandler), cleanup.InNamespace(ns.Name))

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			// Check ConfigMap state
			if tt.cmMatcher != nil {
				err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
				g.Expect(err).To(tt.cmMatcher)
			}

			// Check ClusterRole state and owner references
			if tt.crOwnerRefMatcher != nil {
				var updatedCR rbacv1.ClusterRole
				err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cr), &updatedCR)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(updatedCR.GetOwnerReferences()).To(tt.crOwnerRefMatcher)
			}

			// Check metrics
			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
			if tt.deOwnedMetricsMatcher != nil {
				g.Expect(cleanup.DeownedTotal).Should(WithTransform(testutil.ToFloat64, tt.deOwnedMetricsMatcher))
			}
		})
	}
}

func TestCleanupAction_TypePredicate(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name              string
		typePredicate     func(rr *types.ReconciliationRequest, gvk schema.GroupVersionKind) (bool, error)
		expectedCMDeleted bool
		expectedCRDeleted bool
	}{
		{
			name: "should only delete ConfigMaps via type predicate",
			typePredicate: func(rr *types.ReconciliationRequest, objGVK schema.GroupVersionKind) (bool, error) {
				return objGVK == gvk.ConfigMap, nil
			},
			expectedCMDeleted: true,
			expectedCRDeleted: false,
		},
		{
			name: "should only delete ClusterRoles via type predicate",
			typePredicate: func(rr *types.ReconciliationRequest, objGVK schema.GroupVersionKind) (bool, error) {
				return objGVK == gvk.ClusterRole, nil
			},
			expectedCMDeleted: false,
			expectedCRDeleted: true,
		},
		{
			name: "should delete nothing when type predicate returns false",
			typePredicate: func(rr *types.ReconciliationRequest, objGVK schema.GroupVersionKind) (bool, error) {
				return false, nil
			},
			expectedCMDeleted: false,
			expectedCRDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)

			CreateTestResource(t, &rr, rr.Instance)

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "predicate-test-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			// Create ClusterRole
			cr := rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "predicate-test-cr",
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			a := cleanup.NewAction(
				cleanup.InNamespace(ns.Name),
				cleanup.WithHandler(cleanup.DefaultDeleteHandler(metav1.DeletePropagationBackground)),
				cleanup.WithTypePredicate(tt.typePredicate),
			)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			// Verify ConfigMap deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
			if tt.expectedCMDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Verify ClusterRole deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cr), &rbacv1.ClusterRole{})
			if tt.expectedCRDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestCleanupAction_HandlerObjectPredicate(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name              string
		objectPredicate   func(rr *types.ReconciliationRequest, obj unstructured.Unstructured) (bool, error)
		expectedCMDeleted bool
		expectedCRDeleted bool
	}{
		{
			name: "should delete resources with specific label via object predicate",
			objectPredicate: func(rr *types.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
				return resources.HasLabel(&obj, "cleanup-test", "yes"), nil
			},
			expectedCMDeleted: true,
			expectedCRDeleted: false,
		},
		{
			name: "should delete nothing when object predicate returns false",
			objectPredicate: func(rr *types.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
				return false, nil
			},
			expectedCMDeleted: false,
			expectedCRDeleted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)

			CreateTestResource(t, &rr, rr.Instance)

			// Create ConfigMap with specific label for object predicate test
			cmLabels := map[string]string{
				labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
			}
			if strings.Contains(tt.name, "specific label") {
				cmLabels["cleanup-test"] = "yes"
			}

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "predicate-test-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: cmLabels,
				},
			}

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			// Create ClusterRole without the label
			cr := rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "predicate-test-cr",
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(successOrNotFound)
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			// Create handler with object predicate
			handler := cleanup.DeleteHandler(
				&cleanup.DeleteHandlerOptions{
					PropagationPolicy: metav1.DeletePropagationBackground,
				},
				cleanup.WithObjectPredicate(tt.objectPredicate),
			)

			a := cleanup.NewAction(
				cleanup.InNamespace(ns.Name),
				cleanup.WithHandler(handler),
			)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			// Verify ConfigMap deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
			if tt.expectedCMDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Verify ClusterRole deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cr), &rbacv1.ClusterRole{})
			if tt.expectedCRDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestCleanupAction_NamespaceFunctions(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		namespaceConfig       cleanup.ActionOption
		expectedNamespace     string
		shouldCleanup         bool
		deletedMetricsMatcher gTypes.GomegaMatcher
	}{
		{
			name:                  "should use static namespace from InNamespace",
			namespaceConfig:       cleanup.InNamespace("test-static-ns"),
			expectedNamespace:     "test-static-ns",
			shouldCleanup:         true,
			deletedMetricsMatcher: BeNumerically("==", 1),
		},
		{
			name: "should use dynamic namespace from InNamespaceFn",
			namespaceConfig: cleanup.InNamespaceFn(func(ctx context.Context, rr *types.ReconciliationRequest) (string, error) {
				return "test-dynamic-ns", nil
			}),
			expectedNamespace:     "test-dynamic-ns",
			shouldCleanup:         true,
			deletedMetricsMatcher: BeNumerically("==", 1),
		},
		{
			name: "should handle namespace function error gracefully",
			namespaceConfig: cleanup.InNamespaceFn(func(ctx context.Context, rr *types.ReconciliationRequest) (string, error) {
				return "", errors.New("namespace resolution failed")
			}),
			expectedNamespace:     "",
			shouldCleanup:         false,
			deletedMetricsMatcher: BeNumerically("==", 0),
		},
		{
			name:                  "should handle default OperatorNamespace error gracefully",
			namespaceConfig:       nil,
			expectedNamespace:     "",
			shouldCleanup:         false,
			deletedMetricsMatcher: BeNumerically("==", 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)

			rr := NewReconciliationRequest(envTest)

			g.Expect(cli.Create(ctx, rr.Instance)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(successOrNotFound)
			})

			targetNamespace := tt.expectedNamespace
			if targetNamespace == "" {
				targetNamespace = xid.New().String()
			}

			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: targetNamespace,
				},
			}

			CreateTestResource(t, &rr, &ns)

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "namespace-fn-test-cm",
					Namespace: targetNamespace,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			var actionOpts []cleanup.ActionOption
			if tt.namespaceConfig != nil {
				actionOpts = append(actionOpts, tt.namespaceConfig)
			}
			actionOpts = append(actionOpts, cleanup.WithHandler(cleanup.DefaultDeleteHandler(metav1.DeletePropagationBackground)))

			a := cleanup.NewAction(actionOpts...)

			err = a(ctx, &rr)
			if tt.shouldCleanup {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
			}

			if tt.shouldCleanup {
				err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
				g.Expect(err).To(expectNotFound)
			} else {
				err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
				g.Expect(err).NotTo(HaveOccurred())
			}

			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
		})
	}
}

func TestCleanupAction_MixedOwnershipScenarios(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                    string
		checkOwnership          *bool
		expectedOwnedDeleted    bool
		expectedNonOwnedDeleted bool
		deletedMetricsMatcher   gTypes.GomegaMatcher
	}{
		{
			name:                    "should only delete owned resources when ownership check enabled",
			checkOwnership:          ptr.To(true),
			expectedOwnedDeleted:    true,
			expectedNonOwnedDeleted: false,
			deletedMetricsMatcher:   BeNumerically("==", 1),
		},
		{
			name:                    "should delete all resources when ownership check disabled",
			checkOwnership:          ptr.To(false),
			expectedOwnedDeleted:    true,
			expectedNonOwnedDeleted: true,
			deletedMetricsMatcher:   BeNumerically("==", 2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)
			g.Expect(cli.Create(ctx, rr.Instance)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(successOrNotFound)
			})

			// Create owned ConfigMap
			ownedCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-owned-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			CreateTestResource(t, &rr, ownedCM, OwnedBy(rr.Instance))

			// Create non-owned ConfigMap
			nonOwnedCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-nonowned-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        xid.New().String(),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			CreateTestResource(t, &rr, nonOwnedCM)

			// Configure handler with test ownership settings
			handlerOptions := cleanup.DeleteHandlerOptions{
				PropagationPolicy: metav1.DeletePropagationBackground,
			}
			if tt.checkOwnership != nil {
				handlerOptions.CheckOwnership = tt.checkOwnership
			}

			a := cleanup.NewAction(
				cleanup.InNamespace(ns.Name),
				cleanup.WithHandler(cleanup.DeleteHandler(&handlerOptions)),
			)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			// Verify owned ConfigMap deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(ownedCM), &corev1.ConfigMap{})
			if tt.expectedOwnedDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Verify non-owned ConfigMap deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(nonOwnedCM), &corev1.ConfigMap{})
			if tt.expectedNonOwnedDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Check metrics
			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
		})
	}
}

func TestCleanupAction_OwnershipAndAnnotationChecking(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                   string
		ownershipCheck         *bool
		managedAnnotationCheck *bool
		addOwnerRef            bool
		managedAnnotationValue string
		expectedDeleted        bool
		deletedMetricsMatcher  gTypes.GomegaMatcher
	}{
		{
			name:                   "should delete owned resources with managed annotation check enabled",
			ownershipCheck:         ptr.To(true),
			managedAnnotationCheck: ptr.To(true),
			addOwnerRef:            true,
			managedAnnotationValue: "true",
			expectedDeleted:        true,
			deletedMetricsMatcher:  BeNumerically("==", 1),
		},
		{
			name:                   "should not delete owned resources when managed annotation is false",
			ownershipCheck:         ptr.To(true),
			managedAnnotationCheck: ptr.To(true),
			addOwnerRef:            true,
			managedAnnotationValue: "false",
			expectedDeleted:        false,
			deletedMetricsMatcher:  BeNumerically("==", 0),
		},
		{
			name:                   "should delete owned resources when managed annotation check is disabled",
			ownershipCheck:         ptr.To(true),
			managedAnnotationCheck: ptr.To(false),
			addOwnerRef:            true,
			managedAnnotationValue: "false",
			expectedDeleted:        true,
			deletedMetricsMatcher:  BeNumerically("==", 1),
		},
		{
			name:                   "should not delete non-owned resources when ownership check enabled",
			ownershipCheck:         ptr.To(true),
			managedAnnotationCheck: ptr.To(true),
			addOwnerRef:            false,
			managedAnnotationValue: "true",
			expectedDeleted:        false,
			deletedMetricsMatcher:  BeNumerically("==", 0),
		},
		{
			name:                   "should delete non-owned resources when ownership check disabled",
			ownershipCheck:         ptr.To(false),
			managedAnnotationCheck: ptr.To(true),
			addOwnerRef:            false,
			managedAnnotationValue: "true",
			expectedDeleted:        true,
			deletedMetricsMatcher:  BeNumerically("==", 1),
		},
		{
			name:                   "should not delete non-owned resources with managed annotation false even when ownership check disabled",
			ownershipCheck:         ptr.To(false),
			managedAnnotationCheck: ptr.To(true),
			addOwnerRef:            false,
			managedAnnotationValue: "false",
			expectedDeleted:        false,
			deletedMetricsMatcher:  BeNumerically("==", 0),
		},
		{
			name:                   "should delete any resource when both checks disabled",
			ownershipCheck:         ptr.To(false),
			managedAnnotationCheck: ptr.To(false),
			addOwnerRef:            false,
			managedAnnotationValue: "false",
			expectedDeleted:        true,
			deletedMetricsMatcher:  BeNumerically("==", 1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)
			g.Expect(cli.Create(ctx, rr.Instance)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(successOrNotFound)
			})

			// Create ConfigMap with appropriate annotations
			cmAnnotations := map[string]string{
				annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
				annotations.InstanceUID:        xid.New().String(),
				annotations.PlatformVersion:    rr.Release.Version.String(),
				annotations.PlatformType:       string(rr.Release.Name),
			}
			if tt.managedAnnotationValue != "" {
				cmAnnotations[annotations.ManagedByODHOperator] = tt.managedAnnotationValue
			}

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ownership-test-cm",
					Namespace:   ns.Name,
					Annotations: cmAnnotations,
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			if tt.addOwnerRef {
				g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
					NotTo(HaveOccurred())
			}

			g.Expect(cli.Create(ctx, &cm)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			// Create delete handler with specific options
			handlerOptions := cleanup.DeleteHandlerOptions{
				PropagationPolicy: metav1.DeletePropagationBackground,
			}
			if tt.ownershipCheck != nil {
				handlerOptions.CheckOwnership = tt.ownershipCheck
			}
			if tt.managedAnnotationCheck != nil {
				handlerOptions.CheckManagedAnnotation = tt.managedAnnotationCheck
			}

			a := cleanup.NewAction(
				cleanup.InNamespace(ns.Name),
				cleanup.WithHandler(cleanup.DeleteHandler(&handlerOptions)),
			)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			// Verify deletion state
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
			if tt.expectedDeleted {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Check metrics
			if tt.deletedMetricsMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.deletedMetricsMatcher))
			}
		})
	}
}

func TestCleanupAction_IdempotentExecution(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := t.Context()
	cli := envTest.Client()

	tests := []struct {
		name                        string
		firstExecutionShouldDelete  bool
		secondExecutionShouldDelete bool
		uidFn                       func(request *types.ReconciliationRequest) string
		firstRunDeletedMatcher      gTypes.GomegaMatcher
		secondRunDeletedMatcher     gTypes.GomegaMatcher
		cyclesMatcher               gTypes.GomegaMatcher
	}{
		{
			name:                        "should not delete resources on second run when annotations match",
			firstExecutionShouldDelete:  false,
			secondExecutionShouldDelete: false,
			uidFn:                       uuidFromInstance,
			firstRunDeletedMatcher:      BeNumerically("==", 0),
			secondRunDeletedMatcher:     BeNumerically("==", 0),
			cyclesMatcher:               BeNumerically("==", 2),
		},
		{
			name:                        "should delete resources once and be idempotent on second run",
			firstExecutionShouldDelete:  true,
			secondExecutionShouldDelete: false,
			uidFn:                       func(rr *types.ReconciliationRequest) string { return xid.New().String() },
			firstRunDeletedMatcher:      BeNumerically("==", 1),
			secondRunDeletedMatcher:     BeNumerically("==", 1),
			cyclesMatcher:               BeNumerically("==", 2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup.CyclesTotal.Reset()
			cleanup.CyclesTotal.WithLabelValues("dashboard").Add(0)

			cleanup.DeletedTotal.Reset()
			cleanup.DeletedTotal.WithLabelValues("dashboard").Add(0)

			g := NewWithT(t)
			ns := NewTestNamespace()

			rr := NewReconciliationRequest(envTest)

			CreateTestResource(t, &rr, &ns)
			g.Expect(cli.Create(ctx, rr.Instance)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(successOrNotFound)
			})

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "idempotent-test-cm",
					Namespace: ns.Name,
					Annotations: map[string]string{
						annotations.InstanceGeneration: strconv.FormatInt(rr.Instance.GetGeneration(), 10),
						annotations.InstanceUID:        tt.uidFn(&rr),
						annotations.PlatformVersion:    rr.Release.Version.String(),
						annotations.PlatformType:       string(rr.Release.Name),
					},
					Labels: map[string]string{
						labels.PlatformPartOf: strings.ToLower(componentApi.DashboardKind),
					},
				},
			}

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(successOrNotFound)
			})

			a := cleanup.NewAction(
				cleanup.InNamespace(ns.Name),
				cleanup.WithHandler(cleanup.DefaultDeleteHandler(metav1.DeletePropagationBackground)),
			)

			// First execution
			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.firstRunDeletedMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.firstRunDeletedMatcher))
			}

			// Verify first execution result
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
			if tt.firstExecutionShouldDelete {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// If resource was deleted, recreate it for second run test
			if tt.firstExecutionShouldDelete {
				// Clear resource version to avoid creation error
				cm.ResourceVersion = ""
				// Update annotations to match current instance for idempotency test
				cm.Annotations[annotations.InstanceUID] = string(rr.Instance.GetUID())

				g.Expect(cli.Create(ctx, &cm)).NotTo(HaveOccurred())
			}

			// Second execution
			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.secondRunDeletedMatcher != nil {
				g.Expect(cleanup.DeletedTotal).Should(WithTransform(testutil.ToFloat64, tt.secondRunDeletedMatcher))
			}

			// Verify second execution result
			err = cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
			if tt.secondExecutionShouldDelete {
				g.Expect(err).To(expectNotFound)
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Check that cycles counter increased
			if tt.cyclesMatcher != nil {
				g.Expect(cleanup.CyclesTotal).Should(WithTransform(testutil.ToFloat64, tt.cyclesMatcher))
			}
		})
	}
}
