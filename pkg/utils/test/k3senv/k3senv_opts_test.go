package k3senv_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/k3senv"
)

const testCertDir = "/tmp/certs"

func TestOptions_FunctionalStyle(t *testing.T) {
	scheme := runtime.NewScheme()

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithCertDir(testCertDir),
		k3senv.WithKustomization("/path/to/kustomize1"),
		k3senv.WithKustomizations("/path/to/kustomize2", "/path/to/kustomize3"),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Scheme() != scheme {
		t.Error("scheme not set correctly")
	}

	if env.CertDir() != testCertDir {
		t.Errorf("certDir = %q, want %q", env.CertDir(), testCertDir)
	}

	// We can't check kustomizationPaths directly as it's not exposed
	// The functionality is tested indirectly through Start()
}

func TestOptions_StructStyle(t *testing.T) {
	scheme := runtime.NewScheme()

	env, err := k3senv.New(&k3senv.Options{
		Scheme:             scheme,
		CertDir:            testCertDir,
		KustomizationPaths: []string{"/path/to/kustomize1", "/path/to/kustomize2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Scheme() != scheme {
		t.Error("scheme not set correctly")
	}

	if env.CertDir() != testCertDir {
		t.Errorf("certDir = %q, want %q", env.CertDir(), testCertDir)
	}

	// We can't check kustomizationPaths directly as it's not exposed
	// The functionality is tested indirectly through Start()
}

func TestOptions_MixedStyle(t *testing.T) {
	scheme := runtime.NewScheme()

	env, err := k3senv.New(
		&k3senv.Options{
			Scheme:             scheme,
			KustomizationPaths: []string{"/path/to/kustomize1"},
		},
		k3senv.WithCertDir(testCertDir),
		k3senv.WithKustomization("/path/to/kustomize2"),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Scheme() != scheme {
		t.Error("scheme not set correctly")
	}

	if env.CertDir() != testCertDir {
		t.Errorf("certDir = %q, want %q", env.CertDir(), testCertDir)
	}

	// We can't check kustomizationPaths directly as it's not exposed
	// The functionality is tested indirectly through Start()
}

func TestOptions_ApplyToOptions(t *testing.T) {
	scheme := runtime.NewScheme()

	opt1 := &k3senv.Options{
		Scheme:  scheme,
		CertDir: testCertDir,
	}

	opt2 := &k3senv.Options{
		KustomizationPaths: []string{"/path/to/kustomize"},
	}

	target := &k3senv.Options{}
	opt1.ApplyToOptions(target)
	opt2.ApplyToOptions(target)

	if target.Scheme != scheme {
		t.Error("scheme not applied correctly")
	}

	if target.CertDir != testCertDir {
		t.Errorf("certDir = %q, want %q", target.CertDir, testCertDir)
	}

	if len(target.KustomizationPaths) != 1 {
		t.Errorf("len(kustomizationPaths) = %d, want 1", len(target.KustomizationPaths))
	}
}
