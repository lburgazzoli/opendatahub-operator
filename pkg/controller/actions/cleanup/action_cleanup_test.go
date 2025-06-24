//nolint:dupl,maintidx
package cleanup_test

import (
	"context"
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
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"

	. "github.com/onsi/gomega"
)

var (
	v001 = semver.Version{Major: 0, Minor: 0, Patch: 1}
	v010 = semver.Version{Major: 0, Minor: 1, Patch: 0}
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

//nolint:maintidx
func TestCleanupAction_Delete(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := context.Background()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		version               semver.Version
		generated             bool
		matcher               gTypes.GomegaMatcher
		cyclesMetricsMatcher  gTypes.GomegaMatcher
		deletedMetricsMatcher gTypes.GomegaMatcher
		deownedMetricsMatcher gTypes.GomegaMatcher
		labels                map[string]string
		annotations           map[string]string
		options               []cleanup.ActionOpts
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
			options:               []cleanup.ActionOpts{cleanup.WithLabel("foo", "baz")},
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not delete resources because of unremovable type",
			version:               v001,
			generated:             true,
			matcher:               Not(HaveOccurred()),
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			options:               []cleanup.ActionOpts{cleanup.WithProtectedTypes(gvk.ConfigMap, gvk.ClusterRole)},
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

			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: xid.New().String(),
				},
			}

			g.Expect(cli.Create(ctx, &ns)).
				NotTo(HaveOccurred())

			rr := types.ReconciliationRequest{
				Client: cli,
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
						Version: tt.version,
					},
				},
				Generated: tt.generated,
				Controller: mocks.NewMockController(func(m *mocks.MockController) {
					m.On("GetClient").Return(envTest.Client())
					m.On("GetDynamicClient").Return(envTest.DynamicClient())
					m.On("GetDiscoveryClient").Return(envTest.DiscoveryClient())
					m.On("Owns", mock.Anything).Return(false)
				}),
			}

			g.Expect(cli.Create(ctx, rr.Instance)).
				NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			// should never get deleted
			crd := createTestCRD(id)

			t.Cleanup(func() {
				g.Eventually(func() error {
					return cli.Delete(ctx, &crd)
				}).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
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
				g.Expect(cli.Delete(ctx, &l)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &l, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &l)).
				NotTo(HaveOccurred())

			cm := corev1.ConfigMap{ObjectMeta: *om.DeepCopy()}
			cm.Name = "gc-cm"
			cm.Namespace = ns.Name

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			cr := rbacv1.ClusterRole{ObjectMeta: *om.DeepCopy()}
			cr.Name = "gc-cr"

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			opts := make([]cleanup.ActionOpts, 0, len(tt.options)+1)
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
				ct := testutil.ToFloat64(cleanup.CyclesTotal)
				g.Expect(ct).Should(tt.cyclesMetricsMatcher)
			}
			if tt.deletedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeletedTotal)
				g.Expect(ct).Should(tt.deletedMetricsMatcher)
			}
			if tt.deownedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeownedTotal)
				g.Expect(ct).Should(tt.deownedMetricsMatcher)
			}

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.deletedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeletedTotal)
				g.Expect(ct).Should(tt.deletedMetricsMatcher)
			}
			if tt.deownedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeownedTotal)
				g.Expect(ct).Should(tt.deownedMetricsMatcher)
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

	ctx := context.Background()
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
			nsn := xid.New().String()

			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsn,
				},
			}

			g.Expect(cli.Create(ctx, &ns)).
				NotTo(HaveOccurred())

			rr := types.ReconciliationRequest{
				Client: cli,
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

			g.Expect(cli.Create(ctx, rr.Instance)).
				NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cleanup-own-cm",
					Namespace: nsn,
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
				g.Expect(cli.Delete(ctx, &cm)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			if tt.owned {
				g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
					NotTo(HaveOccurred())
			}

			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			// force background as there's no controller
			tt.options.PropagationPolicy = metav1.DeletePropagationBackground

			a := cleanup.NewAction(
				cleanup.WithHandler(cleanup.DeleteHandler(&tt.options)),
				cleanup.InNamespace(nsn),
			)

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.matcher != nil {
				g.Eventually(func() error {
					return cli.Get(ctx, ctrlCli.ObjectKeyFromObject(&cm), &corev1.ConfigMap{})
				}).Should(
					tt.matcher,
				)
			}
		})
	}
}

//nolint:maintidx
func TestCleanupAction_Deown(t *testing.T) {
	g := NewWithT(t)

	envTest, err := envt.New()
	g.Expect(err).NotTo(HaveOccurred())

	t.Cleanup(func() {
		_ = envTest.Stop()
	})

	ctx := context.Background()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		version               semver.Version
		generated             bool
		ownerRefMatcher       gTypes.GomegaMatcher
		cyclesMetricsMatcher  gTypes.GomegaMatcher
		deletedMetricsMatcher gTypes.GomegaMatcher
		deownedMetricsMatcher gTypes.GomegaMatcher
		labels                map[string]string
		annotations           map[string]string
		options               []cleanup.ActionOpts
		uidFn                 func(request *types.ReconciliationRequest) string
	}{
		{
			name:                  "should remove owner references",
			version:               v001,
			generated:             true,
			ownerRefMatcher:       HaveLen(0), // Should have no owner references after cleanup
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0), // No deletions, only deowning
			deownedMetricsMatcher: BeNumerically("==", 2),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not remove owner references because same annotations",
			version:               v010,
			generated:             true,
			ownerRefMatcher:       HaveLen(1), // Should still have owner reference
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deownedMetricsMatcher: BeNumerically("==", 0),
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
			deownedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should not remove owner references because no generated resources detected",
			version:               v001,
			generated:             false,
			ownerRefMatcher:       HaveLen(1), // Should still have owner reference
			cyclesMetricsMatcher:  BeNumerically("==", 0),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deownedMetricsMatcher: BeNumerically("==", 0),
			uidFn:                 uuidFromInstance,
		},
		{
			name:                  "should remove owner references because of UID mismatch",
			version:               v010,
			generated:             true,
			ownerRefMatcher:       HaveLen(0), // Should have no owner references after cleanup
			cyclesMetricsMatcher:  BeNumerically("==", 1),
			deletedMetricsMatcher: BeNumerically("==", 0),
			deownedMetricsMatcher: BeNumerically("==", 2),
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

			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: xid.New().String(),
				},
			}

			g.Expect(cli.Create(ctx, &ns)).
				NotTo(HaveOccurred())

			rr := types.ReconciliationRequest{
				Client: cli,
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
						Version: tt.version,
					},
				},
				Generated: tt.generated,
				Controller: mocks.NewMockController(func(m *mocks.MockController) {
					m.On("GetClient").Return(envTest.Client())
					m.On("GetDynamicClient").Return(envTest.DynamicClient())
					m.On("GetDiscoveryClient").Return(envTest.DiscoveryClient())
					m.On("Owns", mock.Anything).Return(false)
				}),
			}

			g.Expect(cli.Create(ctx, rr.Instance)).
				NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			// should never get deowned (protected type)
			crd := createTestCRD(id)

			t.Cleanup(func() {
				g.Eventually(func() error {
					return cli.Delete(ctx, &crd)
				}).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
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
				g.Expect(cli.Delete(ctx, &l)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &l, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &l)).
				NotTo(HaveOccurred())

			cm := corev1.ConfigMap{ObjectMeta: *om.DeepCopy()}
			cm.Name = "deown-cm"
			cm.Namespace = ns.Name

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cm)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cm, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cm)).
				NotTo(HaveOccurred())

			cr := rbacv1.ClusterRole{ObjectMeta: *om.DeepCopy()}
			cr.Name = "deown-cr"

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, &cr)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			g.Expect(controllerutil.SetOwnerReference(rr.Instance, &cr, cli.Scheme())).
				NotTo(HaveOccurred())
			g.Expect(cli.Create(ctx, &cr)).
				NotTo(HaveOccurred())

			opts := make([]cleanup.ActionOpts, 0, len(tt.options)+2)
			opts = append(opts, cleanup.InNamespace(ns.Name))
			opts = append(opts, cleanup.WithHandler(cleanup.DeownHandler(
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
				ct := testutil.ToFloat64(cleanup.CyclesTotal)
				g.Expect(ct).Should(tt.cyclesMetricsMatcher)
			}
			if tt.deletedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeletedTotal)
				g.Expect(ct).Should(tt.deletedMetricsMatcher)
			}
			if tt.deownedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeownedTotal)
				g.Expect(ct).Should(tt.deownedMetricsMatcher)
			}

			err = a(ctx, &rr)
			g.Expect(err).NotTo(HaveOccurred())

			if tt.deletedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeletedTotal)
				g.Expect(ct).Should(tt.deletedMetricsMatcher)
			}
			if tt.deownedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeownedTotal)
				g.Expect(ct).Should(tt.deownedMetricsMatcher)
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

	ctx := context.Background()
	cli := envTest.Client()

	tests := []struct {
		name                  string
		cmMatcher             gTypes.GomegaMatcher
		crOwnerRefMatcher     gTypes.GomegaMatcher
		deletedMetricsMatcher gTypes.GomegaMatcher
		deownedMetricsMatcher gTypes.GomegaMatcher
		options               cleanup.DeleteHandlerOptions
		owned                 bool
	}{
		{
			name:                  "should delete ConfigMaps and deown ClusterRoles when owned",
			cmMatcher:             Satisfy(k8serr.IsNotFound), // ConfigMap should be deleted
			crOwnerRefMatcher:     HaveLen(0),                 // ClusterRole should have no owner refs
			deletedMetricsMatcher: BeNumerically("==", 1),     // 1 ConfigMap deleted
			deownedMetricsMatcher: BeNumerically("==", 1),     // 1 ClusterRole deowned
			owned:                 true,
		},
		{
			name:                  "should not process non-owned resources",
			cmMatcher:             Not(HaveOccurred()),    // ConfigMap should exist
			crOwnerRefMatcher:     HaveLen(0),             // ClusterRole has no owner refs anyway
			deletedMetricsMatcher: BeNumerically("==", 0), // No deletions
			deownedMetricsMatcher: BeNumerically("==", 0), // No deowning
			owned:                 false,
		},
		{
			name:                  "should process all resources when configured",
			cmMatcher:             Satisfy(k8serr.IsNotFound), // ConfigMap should be deleted
			crOwnerRefMatcher:     HaveLen(0),                 // ClusterRole should have no owner refs
			deletedMetricsMatcher: BeNumerically("==", 1),     // 1 ConfigMap deleted
			deownedMetricsMatcher: BeNumerically("==", 0),     // 1 ClusterRole had no owner already
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
			nsn := xid.New().String()

			ns := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsn,
				},
			}

			g.Expect(cli.Create(ctx, &ns)).
				NotTo(HaveOccurred())

			rr := types.ReconciliationRequest{
				Client: cli,
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

			g.Expect(cli.Create(ctx, rr.Instance)).
				NotTo(HaveOccurred())

			t.Cleanup(func() {
				g.Expect(cli.Delete(ctx, rr.Instance)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
			})

			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-handler-cm",
					Namespace: nsn,
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
				g.Expect(cli.Delete(ctx, &cm)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
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
				g.Expect(cli.Delete(ctx, &cr)).Should(Or(
					Not(HaveOccurred()),
					MatchError(k8serr.IsNotFound, "IsNotFound"),
				))
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
			deownHandler := cleanup.DeownHandler()

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

			a := cleanup.NewAction(cleanup.WithHandler(customHandler), cleanup.InNamespace(nsn))

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
				ct := testutil.ToFloat64(cleanup.DeletedTotal)
				g.Expect(ct).Should(tt.deletedMetricsMatcher)
			}
			if tt.deownedMetricsMatcher != nil {
				ct := testutil.ToFloat64(cleanup.DeownedTotal)
				g.Expect(ct).Should(tt.deownedMetricsMatcher)
			}
		})
	}
}
