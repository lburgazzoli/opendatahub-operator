package v2_test

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
	v2webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/datasciencecluster/v2"
	v3webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/datasciencecluster/v3"
	dsciv1webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/dscinitialization/v1"
	dsciv2webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"

	. "github.com/onsi/gomega"
)

const maasV2StateAnnotation = "conversion.opendatahub.io/maas-v2-state"

func TestMaaSVersionedAdmission(t *testing.T) {
	g := NewWithT(t)

	ctx, env, teardown := envtestutil.SetupEnvAndClient(t,
		[]envt.RegisterWebhooksFn{
			v2webhook.RegisterWebhooks, v3webhook.RegisterWebhooks,
			dsciv1webhook.RegisterWebhooks, dsciv2webhook.RegisterWebhooks,
		}, nil, envtestutil.DefaultWebhookTimeout)
	t.Cleanup(teardown)

	configureDSCConversion(t, ctx, env)
	createDSCI(g, ctx, env.Client())

	warnings := &maasWarningRecorder{}
	config := rest.CopyConfig(env.Config())
	config.WarningHandler = warnings

	cli, err := client.New(config, client.Options{Scheme: env.Scheme()})
	g.Expect(err).NotTo(HaveOccurred())

	// One environment is shared; each sequential scenario cleans up the singleton.
	for _, scenario := range []struct {
		name string
		run  func(*testing.T, context.Context, client.Client)
	}{
		{name: "v3 storage omits legacy", run: testMaaSStorage},
		{name: "v2 round trip and CEL", run: testMaaSRoundTrip},
		{name: "unrelated updates preserve marker", run: testMaaSUnrelatedUpdates},
		{name: "parent edits retire marker", run: testMaaSParentEdits},
		{name: "canonical removal retires marker", run: testMaaSCanonicalRemoval},
		{name: "canonical defaults", run: testMaaSCanonicalDefaults},
		{name: "native create strips marker", run: testMaaSNativeCreate},
	} {
		name := fmt.Sprintf("scenario: %s", scenario.name)
		t.Run(name, func(t *testing.T) {
			scenario.run(t, ctx, cli)
		})
	}

	warnings.mu.Lock()
	defer warnings.mu.Unlock()

	g.Expect(warnings.messages).To(ContainElement(
		"datasciencecluster.opendatahub.io/v2 DataScienceCluster is deprecated; use datasciencecluster.opendatahub.io/v3 DataScienceCluster"))
	g.Expect(warnings.messages).NotTo(ContainElement(ContainSubstring("modelsAsService is deprecated")))
}

func testMaaSStorage(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	g := NewWithT(t)
	createMaaS(t, ctx, cli, nil)

	v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())

	_, legacyPresent, err := unstructured.NestedFieldNoCopy(v3.Object, "spec", "components", "kserve", "modelsAsService")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(legacyPresent).To(BeFalse())
}

func testMaaSRoundTrip(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	g := NewWithT(t)
	createMaaS(t, ctx, cli, nil)
	v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	g.Expect(nestedString(t, v3.Object, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal("Managed"))
	g.Expect(nestedString(t, v3.Object, "spec", "components", "aigateway", "managementState")).To(Equal("Managed"))
	g.Expect(v3.GetAnnotations()).To(HaveKeyWithValue(maasV2StateAnnotation, "legacy-managed"))
	marker := v3.GetAnnotations()[maasV2StateAnnotation]
	assertLegacyMaaS(t, ctx, cli, "Managed")

	v2 := readMaaS(t, ctx, cli, dscv2.GroupVersion.String())
	g.Expect(nestedString(t, v2.Object, "spec", "components", "aigateway", "managementState")).To(Equal("Managed"))
	g.Expect(nestedString(t, v2.Object, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal("Managed"))
	g.Expect(cli.Update(ctx, v2)).To(Succeed(), "Managed -> Managed must remain valid")
	assertLegacyMaaS(t, ctx, cli, "Managed")

	g.Expect(unstructured.SetNestedField(v2.Object, "Removed", "spec", "components", "kserve", "modelsAsService", "managementState")).To(Succeed())
	v2.SetAnnotations(map[string]string{maasV2StateAnnotation: marker})
	g.Expect(cli.Update(ctx, v2)).To(Succeed(), "Managed -> Removed must ignore stale conversion state")
	assertLegacyMaaS(t, ctx, cli, "Removed")

	v2 = readMaaS(t, ctx, cli, dscv2.GroupVersion.String())
	g.Expect(nestedString(t, v2.Object, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal("Managed"),
		"removing legacy MaaS leaves canonical MaaS migrated until explicitly disabled")
	g.Expect(readMaaS(t, ctx, cli, dscv3.GroupVersion.String()).GetAnnotations()).NotTo(HaveKey(maasV2StateAnnotation))
	g.Expect(cli.Update(ctx, v2)).To(Succeed(), "Removed -> Removed must remain valid")

	g.Expect(unstructured.SetNestedField(v2.Object, "Managed", "spec", "components", "kserve", "modelsAsService", "managementState")).To(Succeed())
	v2.SetAnnotations(map[string]string{maasV2StateAnnotation: marker})
	err := cli.Update(ctx, v2)
	g.Expect(k8serr.IsInvalid(err)).To(BeTrue(), "Removed -> Managed must be rejected by CEL: %v", err)
	g.Expect(err).To(MatchError(ContainSubstring("modelsAsService")))

	v2 = readMaaS(t, ctx, cli, dscv2.GroupVersion.String())
	g.Expect(unstructured.SetNestedField(v2.Object, "Removed", "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Succeed())
	g.Expect(cli.Update(ctx, v2)).To(Succeed(), "canonical MaaS can be disabled explicitly")
	v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	g.Expect(nestedString(t, v3.Object, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal("Removed"))
}

func testMaaSUnrelatedUpdates(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	g := NewWithT(t)
	createMaaS(t, ctx, cli, map[string]any{"managementState": "Managed"})
	v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	marker := v3.GetAnnotations()[maasV2StateAnnotation]
	g.Expect(marker).To(Equal("legacy-managed"))

	for _, mutation := range []struct {
		name  string
		apply func(*unstructured.Unstructured)
	}{
		{"spec", func(dsc *unstructured.Unstructured) {
			g.Expect(unstructured.SetNestedField(dsc.Object, "Managed", "spec", "components", "dashboard", "managementState")).To(Succeed())
		}},
		{"metadata", func(dsc *unstructured.Unstructured) { dsc.SetLabels(map[string]string{"preserved": "yes"}) }},
		{"delete annotation", func(dsc *unstructured.Unstructured) { dsc.SetAnnotations(nil) }},
		{"resubmit marker", func(dsc *unstructured.Unstructured) {
			dsc.SetAnnotations(map[string]string{maasV2StateAnnotation: "legacy-managed"})
		}},
	} {
		v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
		mutation.apply(v3)
		g.Expect(cli.Update(ctx, v3)).To(Succeed(), mutation.name)
		v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
		g.Expect(v3.GetAnnotations()).To(HaveKeyWithValue(maasV2StateAnnotation, marker), mutation.name)
		assertLegacyMaaS(t, ctx, cli, "Managed")
	}

	for _, version := range []string{dscv2.GroupVersion.String(), dscv3.GroupVersion.String()} {
		dsc := readMaaS(t, ctx, cli, version)
		g.Expect(unstructured.SetNestedField(dsc.Object, "Managed", "status", "components", "modelsAsAService", "managementState")).To(Succeed())
		g.Expect(cli.Status().Update(ctx, dsc)).To(Succeed())
		v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
		g.Expect(v3.GetAnnotations()).To(HaveKeyWithValue(maasV2StateAnnotation, marker))
		g.Expect(nestedString(t, v3.Object, "status", "components", "modelsAsAService", "managementState")).To(Equal("Managed"))
		assertLegacyMaaS(t, ctx, cli, "Managed")
	}
}

func testMaaSParentEdits(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	for _, field := range [][]string{
		{"aigateway", "modelsAsAService", "managementState"},
		{"kserve", "managementState"},
		{"aigateway", "managementState"},
	} {
		name := fmt.Sprintf("v3 permanently retires legacy after %s/%s", field[0], field[1])
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			createMaaS(t, ctx, cli, map[string]any{"managementState": "Managed"})
			v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
			marker := v3.GetAnnotations()[maasV2StateAnnotation]
			path := append([]string{"spec", "components"}, field...)

			for _, state := range []string{"Removed", "Managed"} {
				v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
				g.Expect(unstructured.SetNestedField(v3.Object, state, path...)).To(Succeed())
				v3.SetAnnotations(map[string]string{maasV2StateAnnotation: marker})
				g.Expect(cli.Update(ctx, v3)).To(Succeed())
				v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
				g.Expect(v3.GetAnnotations()).NotTo(HaveKey(maasV2StateAnnotation))
				assertLegacyMaaS(t, ctx, cli, "Removed")
			}

			v3.SetAnnotations(map[string]string{maasV2StateAnnotation: marker})
			g.Expect(cli.Update(ctx, v3)).To(Succeed(), "an unrelated update cannot resurrect retired history")
			g.Expect(readMaaS(t, ctx, cli, dscv3.GroupVersion.String()).GetAnnotations()).NotTo(HaveKey(maasV2StateAnnotation))

			v2 := readMaaS(t, ctx, cli, dscv2.GroupVersion.String())
			g.Expect(unstructured.SetNestedField(v2.Object, "Managed", "spec", "components", "kserve", "modelsAsService", "managementState")).To(Succeed())
			v2.SetAnnotations(map[string]string{maasV2StateAnnotation: marker})
			g.Expect(k8serr.IsInvalid(cli.Update(ctx, v2))).To(BeTrue())
		})
	}
}

func testMaaSCanonicalRemoval(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	g := NewWithT(t)
	createMaaS(t, ctx, cli, map[string]any{"managementState": "Managed"})
	v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	marker := v3.GetAnnotations()[maasV2StateAnnotation]
	unstructured.RemoveNestedField(v3.Object, "spec", "components", "aigateway", "modelsAsAService")
	g.Expect(cli.Update(ctx, v3)).To(Succeed())
	v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	g.Expect(v3.GetAnnotations()).NotTo(HaveKey(maasV2StateAnnotation))
	assertLegacyMaaS(t, ctx, cli, "Removed")

	g.Expect(unstructured.SetNestedField(v3.Object, "Managed", "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Succeed())
	v3.SetAnnotations(map[string]string{maasV2StateAnnotation: marker})
	g.Expect(cli.Update(ctx, v3)).To(Succeed())
	g.Expect(readMaaS(t, ctx, cli, dscv3.GroupVersion.String()).GetAnnotations()).NotTo(HaveKey(maasV2StateAnnotation))
	assertLegacyMaaS(t, ctx, cli, "Removed")
}

func testMaaSCanonicalDefaults(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	// Raw omission survives when the defaulter makes no change and skips its
	// serialization/patch step. A typed zero-value MaaS spec serializes as {};
	// an unrelated default can therefore materialize the canonical stanza in a
	// patch. Only then can the API server default its state to Removed. The
	// absent parent has no schema default, and conversion itself applies none.
	for _, tc := range []struct {
		name      string
		canonical map[string]any
		want      string
	}{
		{name: "absent canonical takes legacy fallback", want: "Managed"},
		{name: "present empty canonical defaults to Removed", canonical: map[string]any{}, want: "Removed"},
	} {
		name := fmt.Sprintf("defaulting: %s", tc.name)
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			createMaaS(t, ctx, cli, tc.canonical)
			v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
			g.Expect(nestedString(t, v3.Object, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal(tc.want))
			g.Expect(nestedString(t, v3.Object, "spec", "components", "aigateway", "managementState")).To(Equal("Managed"))
			g.Expect(v3.GetAnnotations()).To(HaveKeyWithValue(maasV2StateAnnotation, "legacy-managed"))
			assertLegacyMaaS(t, ctx, cli, "Managed")

			v2 := readMaaS(t, ctx, cli, dscv2.GroupVersion.String())
			g.Expect(nestedString(t, v2.Object, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal(tc.want))
			g.Expect(cli.Update(ctx, v2)).To(Succeed())
		})
	}
}

func testMaaSNativeCreate(t *testing.T, ctx context.Context, cli client.Client) {
	t.Helper()

	g := NewWithT(t)

	// Capture a valid annotation from a real conversion, then try to reuse it on create.
	legacy := rawLegacyMaaS()
	g.Expect(cli.Create(ctx, legacy)).To(Succeed())
	v3 := readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	g.Expect(v3.GetAnnotations()).To(HaveKey(maasV2StateAnnotation))
	g.Expect(cli.Delete(ctx, legacy)).To(Succeed())
	g.Eventually(func() bool {
		return k8serr.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(legacy), legacy))
	}, "10s", "100ms").Should(BeTrue())

	v3.SetResourceVersion("")
	v3.SetUID("")
	v3.SetCreationTimestamp(metav1.Time{})
	v3.SetManagedFields(nil)
	g.Expect(cli.Create(ctx, v3)).To(Succeed())
	t.Cleanup(func() { g.Expect(cli.Delete(ctx, v3)).To(Succeed()) })

	v3 = readMaaS(t, ctx, cli, dscv3.GroupVersion.String())
	g.Expect(v3.GetAnnotations()).NotTo(HaveKey(maasV2StateAnnotation))
	assertLegacyMaaS(t, ctx, cli, "Removed")
}

func createMaaS(t *testing.T, ctx context.Context, cli client.Client, canonical map[string]any) {
	t.Helper()

	g := NewWithT(t)
	dsc := rawLegacyMaaS()

	if canonical != nil {
		g.Expect(unstructured.SetNestedMap(dsc.Object, canonical, "spec", "components", "aigateway", "modelsAsAService")).To(Succeed())
	}

	g.Expect(cli.Create(ctx, dsc)).To(Succeed())

	t.Cleanup(func() {
		g.Expect(cli.Delete(ctx, dsc)).To(Succeed())
		g.Eventually(func() bool {
			return k8serr.IsNotFound(cli.Get(ctx, client.ObjectKeyFromObject(dsc), dsc))
		}, "10s", "100ms").Should(BeTrue())
	})
}

func readMaaS(t *testing.T, ctx context.Context, cli client.Client, version string) *unstructured.Unstructured {
	t.Helper()

	dsc := &unstructured.Unstructured{}
	dsc.SetAPIVersion(version)
	dsc.SetKind("DataScienceCluster")

	NewWithT(t).Expect(cli.Get(ctx, client.ObjectKey{Name: "maas-compatibility"}, dsc)).To(Succeed())

	return dsc
}

func assertLegacyMaaS(t *testing.T, ctx context.Context, cli client.Client, state string) {
	t.Helper()

	g := NewWithT(t)
	dsc := readMaaS(t, ctx, cli, dscv2.GroupVersion.String())
	g.Expect(nestedString(t, dsc.Object, "spec", "components", "kserve", "modelsAsService", "managementState")).To(Equal(state))
}

// Raw maps keep an absent canonical object distinct from a present empty object.
func rawLegacyMaaS() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": dscv2.GroupVersion.String(),
		"kind":       "DataScienceCluster",
		"metadata":   map[string]any{"name": "maas-compatibility"},
		"spec": map[string]any{"components": map[string]any{
			"kserve": map[string]any{
				"managementState": "Managed",
				"modelsAsService": map[string]any{"managementState": "Managed"},
			},
		}},
	}}
}

func nestedString(t *testing.T, object map[string]any, path ...string) string {
	t.Helper()

	g := NewWithT(t)

	value, found, err := unstructured.NestedString(object, path...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(found).To(BeTrue(), "missing field %v", path)

	return value
}

func configureDSCConversion(t *testing.T, ctx context.Context, env *envt.EnvT) {
	t.Helper()

	g := NewWithT(t)

	extensionsClient, err := apiextensionsclientset.NewForConfig(env.Config())
	g.Expect(err).NotTo(HaveOccurred())

	crdClient := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()

	crd, err := crdClient.Get(ctx, "datascienceclusters.datasciencecluster.opendatahub.io", metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	options := env.Env.WebhookInstallOptions
	url := "https://" + net.JoinHostPort(options.LocalServingHost, strconv.Itoa(options.LocalServingPort)) + "/convert"
	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             &apiextensionsv1.WebhookClientConfig{URL: &url, CABundle: options.LocalServingCAData},
			ConversionReviewVersions: []string{"v1"},
		},
	}

	_, err = crdClient.Update(ctx, crd, metav1.UpdateOptions{})
	g.Expect(err).NotTo(HaveOccurred())
}

type maasWarningRecorder struct {
	mu       sync.Mutex
	messages []string
}

func (w *maasWarningRecorder) HandleWarningHeader(_ int, _ string, message string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, message)
}
