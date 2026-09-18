package v2_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
	v2webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/datasciencecluster/v2"
	v3webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/datasciencecluster/v3"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"

	. "github.com/onsi/gomega"
)

func TestConversionReviewBothDirections(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	_, env, teardown := envtestutil.SetupEnvAndClient(t,
		[]envt.RegisterWebhooksFn{v2webhook.RegisterWebhooks, v3webhook.RegisterWebhooks},
		nil, envtestutil.DefaultWebhookTimeout)
	t.Cleanup(teardown)

	webhookOptions := env.Env.WebhookInstallOptions
	roots := x509.NewCertPool()
	g.Expect(roots.AppendCertsFromPEM(webhookOptions.LocalServingCAData)).To(BeTrue())
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	t.Cleanup(client.CloseIdleConnections)
	url := "https://" + net.JoinHostPort(webhookOptions.LocalServingHost, strconv.Itoa(webhookOptions.LocalServingPort)) + "/convert"

	for _, direction := range []struct{ source, target string }{
		{dscv2.GroupVersion.String(), dscv3.GroupVersion.String()},
		{dscv3.GroupVersion.String(), dscv2.GroupVersion.String()},
	} {
		for _, count := range []int{1, 3} {
			name := fmt.Sprintf("%s-to-%s-count-%d", direction.source, direction.target, count)

			t.Run(name, func(t *testing.T) {
				t.Parallel()
				g := NewWithT(t)
				objects := make([]runtime.RawExtension, count)
				for i := range objects {
					object := &dscv2.DataScienceCluster{
						TypeMeta: metav1.TypeMeta{APIVersion: direction.source, Kind: "DataScienceCluster"},
						ObjectMeta: metav1.ObjectMeta{
							Name:        "dsc-" + strconv.Itoa(i),
							Labels:      map[string]string{"source": "kept"},
							Annotations: map[string]string{"annotation": "kept"},
							Finalizers:  []string{"example/finalizer"},
						},
					}
					object.Spec.Components.Dashboard.ManagementState = operatorv1.Managed

					switch {
					case direction.source == dscv2.GroupVersion.String():
						object.Annotations[maasV2StateAnnotation] = "stale-value"
					case i == 1:
						object.Annotations[maasV2StateAnnotation] = "legacy-managed"
					}

					object.Spec.Components.Kserve.ManagementState = operatorv1.Managed
					object.Spec.Components.Kserve.ModelsAsService.ManagementState = operatorv1.Managed //nolint:staticcheck
					if i == 1 {
						object.Spec.Components.Kserve.ModelsAsService.ManagementState = operatorv1.Removed //nolint:staticcheck
						object.Spec.Components.AIGateway.ManagementState = operatorv1.Removed
					}

					switch {
					case i == 2:
						object.Spec.Components.AIGateway.ModelsAsAService.ManagementState = operatorv1.Removed
					case i > 0 || direction.source == dscv3.GroupVersion.String():
						object.Spec.Components.AIGateway.ModelsAsAService.ManagementState = operatorv1.Managed
					}

					object.Status.Components.Dashboard.ManagementState = operatorv1.Managed
					object.Status.Components.ModelsAsAService.ManagementState = operatorv1.Removed
					object.Status.ErrorMessage = "status-kept"
					object.Status.Conditions = []common.Condition{{
						Type: "ModelsAsAServiceReady", Status: metav1.ConditionTrue,
						Reason: "Ready", LastTransitionTime: metav1.Now(),
					}}

					payload, err := json.Marshal(object)
					g.Expect(err).NotTo(HaveOccurred())
					if direction.source == dscv3.GroupVersion.String() {
						var source map[string]any
						g.Expect(json.Unmarshal(payload, &source)).To(Succeed())
						unstructured.RemoveNestedField(source, "spec", "components", "kserve", "modelsAsService")
						payload, err = json.Marshal(source)
						g.Expect(err).NotTo(HaveOccurred())
					}
					objects[i] = runtime.RawExtension{Raw: payload}
				}

				review := apiextensionsv1.ConversionReview{
					TypeMeta: metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "ConversionReview"},
					Request: &apiextensionsv1.ConversionRequest{
						UID: types.UID("request-uid"), DesiredAPIVersion: direction.target, Objects: objects,
					},
				}
				result := postConversionReview(t, client, url, review)
				g.Expect(result.Response.UID).To(Equal(types.UID("request-uid")))
				g.Expect(result.Response.Result.Status).To(Equal(metav1.StatusSuccess))
				g.Expect(result.Response.ConvertedObjects).To(HaveLen(count))

				for i, converted := range result.Response.ConvertedObjects {
					var source, target map[string]any
					g.Expect(json.Unmarshal(objects[i].Raw, &source)).To(Succeed())
					g.Expect(json.Unmarshal(converted.Raw, &target)).To(Succeed())
					g.Expect(target["apiVersion"]).To(Equal(direction.target))
					if direction.target == dscv3.GroupVersion.String() {
						_, found, err := unstructured.NestedFieldNoCopy(target, "spec", "components", "kserve", "modelsAsService")
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(found).To(BeFalse())

						want := "Managed"
						parent := "Managed"
						switch i {
						case 1:
							parent = "Removed"
						case 2:
							want = "Removed"
						}

						g.Expect(nestedString(t, target, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Equal(want))
						g.Expect(nestedString(t, target, "spec", "components", "aigateway", "managementState")).To(Equal(parent))

						if i == 1 {
							_, found, err := unstructured.NestedFieldNoCopy(target, "metadata", "annotations", maasV2StateAnnotation)
							g.Expect(err).NotTo(HaveOccurred())
							g.Expect(found).To(BeFalse(), "forward conversion clears stale markers when legacy is Removed")
						} else {
							g.Expect(nestedString(t, target, "metadata", "annotations", maasV2StateAnnotation)).To(Equal("legacy-managed"))
						}

						unstructured.RemoveNestedField(target, "metadata", "annotations", maasV2StateAnnotation)
						unstructured.RemoveNestedField(source, "spec", "components", "kserve", "modelsAsService")
						g.Expect(unstructured.SetNestedField(source, want, "spec", "components", "aigateway", "modelsAsAService", "managementState")).To(Succeed())
						g.Expect(unstructured.SetNestedField(source, parent, "spec", "components", "aigateway", "managementState")).To(Succeed())
					} else {
						legacy := "Removed"
						if i == 1 {
							legacy = "Managed"
						}
						g.Expect(nestedString(t, target, "spec", "components", "kserve", "modelsAsService", "managementState")).To(Equal(legacy))
						unstructured.RemoveNestedField(target, "spec", "components", "kserve", "modelsAsService")
					}

					unstructured.RemoveNestedField(source, "metadata", "annotations", maasV2StateAnnotation)
					delete(source, "apiVersion")
					delete(target, "apiVersion")
					g.Expect(target).To(Equal(source), "conversion must preserve unrelated metadata, spec, and status")
				}
			})
		}
	}

	for _, marker := range []string{"", "Managed", "{}", "unknown"} {
		name := "reject unknown marker " + marker

		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			object := envtestutil.NewDSC("invalid-marker")
			object.Annotations = map[string]string{maasV2StateAnnotation: marker}
			payload, err := json.Marshal(object)
			g.Expect(err).NotTo(HaveOccurred())
			result := postConversionReview(t, client, url, apiextensionsv1.ConversionReview{
				TypeMeta: metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "ConversionReview"},
				Request: &apiextensionsv1.ConversionRequest{
					UID: types.UID("invalid-marker"), DesiredAPIVersion: dscv2.GroupVersion.String(),
					Objects: []runtime.RawExtension{{Raw: payload}},
				},
			})
			g.Expect(result.Response.UID).To(Equal(types.UID("invalid-marker")))
			g.Expect(result.Response.Result.Status).To(Equal(metav1.StatusFailure))
			g.Expect(result.Response.Result.Message).To(ContainSubstring("invalid MaaS v2 provenance marker"))
			g.Expect(result.Response.ConvertedObjects).To(BeEmpty())
		})
	}
}

func postConversionReview(
	t *testing.T, client *http.Client, url string, review apiextensionsv1.ConversionReview,
) apiextensionsv1.ConversionReview {
	t.Helper()
	g := NewWithT(t)
	requestBytes, err := json.Marshal(review)
	g.Expect(err).NotTo(HaveOccurred())

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(requestBytes))
	g.Expect(err).NotTo(HaveOccurred())
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(response.StatusCode).To(Equal(http.StatusOK))

	var result apiextensionsv1.ConversionReview
	g.Expect(json.NewDecoder(response.Body).Decode(&result)).To(Succeed())
	g.Expect(response.Body.Close()).To(Succeed())
	g.Expect(result.Response).NotTo(BeNil())
	return result
}
