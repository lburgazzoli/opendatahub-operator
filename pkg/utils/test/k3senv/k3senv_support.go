package k3senv

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/mdelapenya/tlscert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"
)

// CertData holds certificate data for webhooks.
type CertData struct {
	CACert     []byte
	ServerCert []byte
	ServerKey  []byte
}

func (cd *CertData) CABundle() []byte {
	return []byte(base64.StdEncoding.EncodeToString(cd.CACert))
}

func generateCertificates(certDir string, validity time.Duration) (*CertData, error) {
	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Generate CA certificate
	caCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "ca",
		Host:      "k3senv-ca",
		ValidFor:  validity,
		IsCA:      true,
		ParentDir: certDir,
	})

	// Generate server certificate signed by CA using configured SANs
	serverCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "tls",
		Host:      strings.Join(CertificateSANs, ","),
		ValidFor:  validity,
		Parent:    caCert,
		ParentDir: certDir,
	})

	// Ensure certificates were generated
	if caCert == nil || serverCert == nil {
		return nil, errors.New("failed to generate certificates")
	}

	// Read the generated files (tlscert uses cert-<Name>.pem and key-<Name>.pem format)
	caCertPEM, err := os.ReadFile(filepath.Join(certDir, CACertFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	serverCertPEM, err := os.ReadFile(filepath.Join(certDir, CertFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to read server cert: %w", err)
	}

	serverKeyPEM, err := os.ReadFile(filepath.Join(certDir, KeyFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to read server key: %w", err)
	}

	return &CertData{
		CACert:     caCertPEM,
		ServerCert: serverCertPEM,
		ServerKey:  serverKeyPEM,
	}, nil
}

// determineConvertibleCRDs determines which CRDs need conversion webhooks based on the scheme.
// It follows the same logic as controller-runtime's envtest.modifyConversionWebhooks.
func determineConvertibleCRDs(
	crds []unstructured.Unstructured,
	scheme *runtime.Scheme,
) ([]unstructured.Unstructured, error) {
	// Build map of convertible GroupKinds using conversion.IsConvertible
	convertibles := map[schema.GroupKind]struct{}{}
	for gvk := range scheme.AllKnownTypes() {
		obj, err := scheme.New(gvk)
		if err != nil {
			return nil, err
		}
		if ok, err := conversion.IsConvertible(scheme, obj); ok && err == nil {
			convertibles[gvk.GroupKind()] = struct{}{}
		}
	}

	// Filter CRDs that are convertible
	var convertibleCRDs []unstructured.Unstructured
	for _, crd := range crds {
		// Extract group and kind from CRD spec
		group, found, err := unstructured.NestedString(crd.Object, "spec", "group")
		if err != nil || !found {
			return nil, fmt.Errorf("failed to extract group from CRD %s: %w", crd.GetName(), err)
		}

		kind, found, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if err != nil || !found {
			return nil, fmt.Errorf("failed to extract kind from CRD %s: %w", crd.GetName(), err)
		}

		if _, ok := convertibles[schema.GroupKind{Group: group, Kind: kind}]; ok {
			convertibleCRDs = append(convertibleCRDs, crd)
		}
	}

	return convertibleCRDs, nil
}

func checkWebhookHealth(ctx context.Context, hostPort string) error {
	// Create HTTP client with custom TLS config to accept self-signed certificates
	client := &http.Client{
		Timeout: WebhookHealthCheckTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// Required for testing webhooks with self-signed certificates
				//nolint:gosec
				InsecureSkipVerify: true,
			},
		},
	}

	// Make a request to the webhook server root path
	// We expect a 404 (no handler at /) or 400 (bad request), not connection errors or 500s
	url := fmt.Sprintf("https://%s/", hostPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to webhook server: %w", err)
	}
	defer resp.Body.Close()

	// Accept any response code that indicates the server is responding
	// 404, 400, 405 are all acceptable - they mean the webhook is running
	// 500+ would indicate a problem
	if resp.StatusCode >= 500 {
		return fmt.Errorf("webhook server returned error status: %d", resp.StatusCode)
	}

	return nil
}

// ApplyJQTransform applies a JQ expression to transform an unstructured object.
// It parses the provided JQ expression (with optional format arguments) and applies
// it to the object, updating the object's content with the transformation result.
func ApplyJQTransform(obj *unstructured.Unstructured, expression string, args ...interface{}) error {
	query, err := gojq.Parse(fmt.Sprintf(expression, args...))
	if err != nil {
		return fmt.Errorf("failed to parse jq expression: %w", err)
	}

	result, ok := query.Run(obj.Object).Next()
	if !ok || result == nil {
		return nil
	}

	if err, ok := result.(error); ok {
		return fmt.Errorf("jq execution error: %w", err)
	}

	transformed, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map[string]interface{}, got %T", result)
	}

	obj.SetUnstructuredContent(transformed)

	return nil
}

// extractNames extracts the names from a slice of unstructured objects.
func extractNames(objs []unstructured.Unstructured) []string {
	names := make([]string, len(objs))
	for i := range objs {
		names[i] = objs[i].GetName()
	}
	return names
}
