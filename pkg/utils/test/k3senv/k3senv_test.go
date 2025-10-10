package k3senv_test

import (
	"context"
	"testing"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/k3senv"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/onsi/gomega"
)

func TestK3sEnv_GetKubeconfig_Success(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New(k3senv.WithCertDir(t.TempDir()))
	g.Expect(err).ShouldNot(HaveOccurred())

	// Start the cluster
	err = env.Start(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	// Get kubeconfig
	kubeconfigData, err := env.GetKubeconfig(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(kubeconfigData).ToNot(BeEmpty())

	// Verify it's valid kubeconfig by parsing it
	config, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(config).ToNot(BeNil())
	g.Expect(config.Clusters).ToNot(BeEmpty())
	g.Expect(config.AuthInfos).ToNot(BeEmpty())
	g.Expect(config.Contexts).ToNot(BeEmpty())
}

func TestK3sEnv_GetKubeconfig_BeforeStart(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Try to get kubeconfig before Start()
	_, err = env.GetKubeconfig(ctx)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("cluster not started"))
}

func TestK3sEnv_GetKubeconfig_MatchesConfig(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	env, err := k3senv.New(k3senv.WithCertDir(t.TempDir()))
	g.Expect(err).ShouldNot(HaveOccurred())

	// Start the cluster
	err = env.Start(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())
	t.Cleanup(func() {
		_ = env.Stop(ctx)
	})

	// Get kubeconfig and parse it
	kubeconfigData, err := env.GetKubeconfig(ctx)
	g.Expect(err).ShouldNot(HaveOccurred())

	config, err := clientcmd.Load(kubeconfigData)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Build REST config from kubeconfig
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	g.Expect(err).ShouldNot(HaveOccurred())

	// Compare with env.Config()
	envConfig := env.Config()
	g.Expect(restConfig.Host).To(Equal(envConfig.Host))
	g.Expect(restConfig.CAData).To(Equal(envConfig.CAData))
}
