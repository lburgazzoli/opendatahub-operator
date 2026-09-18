package v2_test

import (
	"net"
	"strconv"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
	v2webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/datasciencecluster/v2"
	v3webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/datasciencecluster/v3"
	dsciv1webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/dscinitialization/v1"
	dsciv2webhook "github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"

	. "github.com/onsi/gomega"
)

func TestV2ClientAgainstV3Storage(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	ctx, env, teardown := envtestutil.SetupEnvAndClient(t,
		[]envt.RegisterWebhooksFn{v3webhook.RegisterWebhooks, dsciv1webhook.RegisterWebhooks, v2webhook.RegisterWebhooks, dsciv2webhook.RegisterWebhooks},
		nil, envtestutil.DefaultWebhookTimeout)
	t.Cleanup(teardown)
	cli := env.Client()
	createDSCI(g, ctx, cli)
	extensionsClient, err := apiextensionsclientset.NewForConfig(env.Config())
	g.Expect(err).NotTo(HaveOccurred())
	crdClient := extensionsClient.ApiextensionsV1().CustomResourceDefinitions()
	crd, err := crdClient.Get(ctx, "datascienceclusters.datasciencecluster.opendatahub.io", metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	webhookOptions := env.Env.WebhookInstallOptions
	url := "https://" + net.JoinHostPort(webhookOptions.LocalServingHost, strconv.Itoa(webhookOptions.LocalServingPort)) + "/convert"
	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             &apiextensionsv1.WebhookClientConfig{URL: &url, CABundle: webhookOptions.LocalServingCAData},
			ConversionReviewVersions: []string{"v1"},
		},
	}
	_, err = crdClient.Update(ctx, crd, metav1.UpdateOptions{})
	g.Expect(err).NotTo(HaveOccurred())

	v2 := envtestutil.NewDSCV2("v2-client-v3-storage")
	v2.Labels = map[string]string{"conversion": "preserved"}
	v2.Spec.Components.Dashboard.ManagementState = operatorv1.Managed
	g.Expect(cli.Create(ctx, v2)).To(Succeed())
	key := client.ObjectKeyFromObject(v2)
	v3 := &dscv3.DataScienceCluster{}
	g.Expect(cli.Get(ctx, key, v3)).To(Succeed())
	g.Expect(v3.Labels).To(HaveKeyWithValue("conversion", "preserved"))
	g.Expect(v3.Spec.Components.Dashboard.ManagementState).To(Equal(operatorv1.Managed))

	v3.Status.Components.Dashboard.ManagementState = operatorv1.Managed
	v3.Status.Conditions = append(v3.Status.Conditions, common.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Test", LastTransitionTime: metav1.Now()})
	g.Expect(cli.Status().Update(ctx, v3)).To(Succeed())
	g.Expect(cli.Get(ctx, key, v2)).To(Succeed())
	g.Expect(v2.Status.Components.Dashboard.ManagementState).To(Equal(operatorv1.Managed))
	g.Expect(v2.Status.Conditions).To(HaveLen(1))
	v2.Spec.Components.Workbenches.ManagementState = operatorv1.Removed
	g.Expect(cli.Update(ctx, v2)).To(Succeed())
	g.Expect(cli.Get(ctx, key, v3)).To(Succeed())
	g.Expect(v3.Spec.Components.Workbenches.ManagementState).To(Equal(operatorv1.Removed))
	g.Expect(v3.Status.Components.Dashboard.ManagementState).To(Equal(operatorv1.Managed))
	g.Expect(v3.Status.Conditions).To(HaveLen(1))

	g.Expect(v3.ResourceVersion).NotTo(BeEmpty())
	crd, err = crdClient.Get(ctx, "datascienceclusters.datasciencecluster.opendatahub.io", metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(crd.Status.StoredVersions).To(Equal([]string{dscv3.GroupVersion.Version}))
	g.Expect(crd.Spec.Conversion.Strategy).To(Equal(apiextensionsv1.WebhookConverter))
}
