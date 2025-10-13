//go:build !nowebhook

package dashboard_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/rs/xid"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"

	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/dashboard"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/scheme"

	. "github.com/onsi/gomega"
)

// createTestObject creates an unstructured test object with the specified GVK, name, and namespace,
// and persists it to the cluster via the provided client.
func createTestObject(
	ctx context.Context,
	cli client.Client,
	gvk schema.GroupVersionKind,
	name string,
	namespace string,
) (*unstructured.Unstructured, error) {
	obj := resources.GvkToUnstructured(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	if err := cli.Create(ctx, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

func TestValidator_K3sEnv_Integration(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	// Create scheme with all required types
	s, err := scheme.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Register deprecated dashboard types
	scheme.RegisterUnstructuredTypes(s, gvk.DashboardAcceleratorProfile)
	scheme.RegisterUnstructuredTypes(s, gvk.DashboardHardwareProfile)

	env, err := k3senv.New(&k3senv.Options{
		Scheme: s,
		Certificate: k3senv.CertificateConfig{
			Path: t.TempDir(),
		},
		Manifest: k3senv.ManifestConfig{
			Paths: []string{
				"config/crd/bases",
				"config/webhook/manifests.yaml",
			},
			Objects: []client.Object{
				envtestutil.MockAcceleratorProfileCRD(),
				envtestutil.MockDashboardHardwareProfileCRD(),
			},
		},
		Logger: t,
	})

	g.Expect(err).ShouldNot(HaveOccurred())

	// Start k3senv
	err = env.Start(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Add teardown
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	ns := envtestutil.NewNamespace("test-"+xid.New().String(), nil)
	err = env.Client().Create(ctx, ns)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Now create test objects for update/delete tests (after CRDs exist, before webhooks are active)
	apForUpdate, err := createTestObject(ctx, env.Client(), gvk.DashboardAcceleratorProfile, "test-ap-update-"+xid.New().String(), ns.Name)
	g.Expect(err).ShouldNot(HaveOccurred())

	apForDelete, err := createTestObject(ctx, env.Client(), gvk.DashboardAcceleratorProfile, "test-ap-delete-"+xid.New().String(), ns.Name)
	g.Expect(err).ShouldNot(HaveOccurred())

	hpForUpdate, err := createTestObject(ctx, env.Client(), gvk.DashboardHardwareProfile, "test-hp-update-"+xid.New().String(), ns.Name)
	g.Expect(err).ShouldNot(HaveOccurred())

	hpForDelete, err := createTestObject(ctx, env.Client(), gvk.DashboardHardwareProfile, "test-hp-delete-"+xid.New().String(), ns.Name)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Create manager
	mgr, err := ctrl.NewManager(env.Config(), ctrl.Options{
		Scheme:        s,
		WebhookServer: env.WebhookServer(),
	})
	g.Expect(err).ShouldNot(HaveOccurred())

	err = dashboard.RegisterWebhooks(mgr)
	g.Expect(err).ShouldNot(HaveOccurred())

	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Logf("manager stopped: %v", err)
		}
	}()

	// Install webhooks and patches CRDs (will wait for webhook server to be ready internally)
	err = env.InstallWebhooks(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Run test cases
	tmaps := []struct {
		deprecatedGVK  schema.GroupVersionKind
		replacementGVK schema.GroupVersionKind
		updateObject   client.Object
		deleteObject   client.Object
	}{
		{
			deprecatedGVK:  gvk.DashboardAcceleratorProfile,
			replacementGVK: gvk.HardwareProfile,
			updateObject:   apForUpdate,
			deleteObject:   apForDelete,
		},
		{
			deprecatedGVK:  gvk.DashboardHardwareProfile,
			replacementGVK: gvk.HardwareProfile,
			updateObject:   hpForUpdate,
			deleteObject:   hpForDelete,
		},
	}

	for _, item := range tmaps {
		msg := fmt.Sprintf("%s/%s is not supported, please use %s/%s",
			item.deprecatedGVK.Group,
			item.deprecatedGVK.Kind,
			item.replacementGVK.Group,
			item.replacementGVK.Kind,
		)

		t.Run("Should deny creation of "+item.deprecatedGVK.Kind, func(t *testing.T) {
			g := NewWithT(t)

			_, err := createTestObject(ctx, env.Client(), item.deprecatedGVK, "test-create-"+xid.New().String(), ns.Name)
			g.Expect(err).To(HaveOccurred())

			statusErr := &k8serr.StatusError{}
			ok := errors.As(err, &statusErr)
			g.Expect(ok).To(BeTrue(), "Expected error to be of type StatusError")

			g.Expect(statusErr.Status().Code).To(Equal(int32(http.StatusForbidden)))
			g.Expect(statusErr.Status().Message).To(ContainSubstring(msg))
		})

		t.Run("Should deny update of "+item.deprecatedGVK.Kind, func(t *testing.T) {
			g := NewWithT(t)

			// Update the pre-created object
			err := env.Client().Update(ctx, item.updateObject)

			g.Expect(err).To(HaveOccurred())

			statusErr := &k8serr.StatusError{}
			ok := errors.As(err, &statusErr)
			g.Expect(ok).To(BeTrue(), "Expected error to be of type StatusError")

			g.Expect(statusErr.Status().Code).To(Equal(int32(http.StatusForbidden)))
			g.Expect(statusErr.Status().Message).To(ContainSubstring(msg))
		})

		t.Run("Should allow deletion of "+item.deprecatedGVK.Kind, func(t *testing.T) {
			g := NewWithT(t)

			// Delete the pre-created object
			err := env.Client().Delete(ctx, item.deleteObject)

			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
