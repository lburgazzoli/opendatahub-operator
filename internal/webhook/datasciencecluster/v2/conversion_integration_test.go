package v2_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

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
			t.Run(direction.source+"-to-"+direction.target+"-count-"+strconv.Itoa(count), func(t *testing.T) {
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
					object.Spec.Components.AIGateway.ModelsAsAService.ManagementState = operatorv1.Removed
					object.Status.Components.Dashboard.ManagementState = operatorv1.Managed
					object.Status.Components.ModelsAsAService.ManagementState = operatorv1.Removed
					object.Status.ErrorMessage = "status-kept"
					payload, err := json.Marshal(object)
					g.Expect(err).NotTo(HaveOccurred())
					objects[i] = runtime.RawExtension{Raw: payload}
				}
				review := apiextensionsv1.ConversionReview{
					TypeMeta: metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "ConversionReview"},
					Request: &apiextensionsv1.ConversionRequest{
						UID: types.UID("request-uid"), DesiredAPIVersion: direction.target, Objects: objects,
					},
				}
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
				g.Expect(result.Response.UID).To(Equal(types.UID("request-uid")))
				g.Expect(result.Response.Result.Status).To(Equal(metav1.StatusSuccess))
				g.Expect(result.Response.ConvertedObjects).To(HaveLen(count))
				for i, converted := range result.Response.ConvertedObjects {
					var source, target map[string]any
					g.Expect(json.Unmarshal(objects[i].Raw, &source)).To(Succeed())
					g.Expect(json.Unmarshal(converted.Raw, &target)).To(Succeed())
					g.Expect(target["apiVersion"]).To(Equal(direction.target))
					delete(source, "apiVersion")
					delete(target, "apiVersion")
					g.Expect(target).To(Equal(source), "conversion must preserve metadata, spec, and status")
				}
			})
		}
	}
}
