package compare_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
)

// The v2 hashes were captured independently before adding v3. They must never
// be regenerated from a proposed v3 schema change.
type baseline map[string]string

type conversionCase struct {
	CaseID                 string          `json:"case_id"`
	V2Path                 string          `json:"v2_path"`
	V3Path                 string          `json:"v3_path"`
	OpenAPIPaths           []string        `json:"openapi_paths"`
	ChangedSemantics       string          `json:"changed_semantics"`
	V2Object               json.RawMessage `json:"v2_object"`
	V3Object               json.RawMessage `json:"v3_object"`
	ExpectedV2             json.RawMessage `json:"expected_v2"`
	ExpectedV3             json.RawMessage `json:"expected_v3"`
	ExpectedV2RoundTrip    json.RawMessage `json:"expected_v2_round_trip,omitempty"`
	ExpectedV3RoundTrip    json.RawMessage `json:"expected_v3_round_trip,omitempty"`
	NormalizationRationale string          `json:"normalization_rationale,omitempty"`
}

var requiredCases = []string{"forward", "backward", "v2-round-trip", "v3-round-trip"}

func repoRoot(t *testing.T) string {
	t.Helper()
	g := NewWithT(t)
	_, file, _, ok := runtime.Caller(0)
	g.Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func readSchema(t *testing.T, path, version string) map[string]any {
	t.Helper()
	g := NewWithT(t)
	data, err := os.ReadFile(path)
	g.Expect(err).NotTo(HaveOccurred())
	var crd map[string]any
	g.Expect(yaml.Unmarshal(data, &crd)).To(Succeed())
	spec := crd["spec"].(map[string]any)
	for _, item := range spec["versions"].([]any) {
		v := item.(map[string]any)
		if v["name"] == version {
			return v["schema"].(map[string]any)["openAPIV3Schema"].(map[string]any)
		}
	}
	t.Fatalf("%s has no %s schema", path, version)
	return nil
}

func canonical(t *testing.T, value any) []byte {
	t.Helper()
	g := NewWithT(t)
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	g.Expect(encoder.Encode(value)).To(Succeed())
	return out.Bytes()
}

func schemaDifferences(a, b any, path string) []string {
	if reflect.DeepEqual(a, b) {
		return nil
	}
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if !aok || !bok {
		return []string{path}
	}
	keys := make(map[string]struct{}, len(am)+len(bm))
	for k := range am {
		keys[k] = struct{}{}
	}
	for k := range bm {
		keys[k] = struct{}{}
	}
	var diff []string
	for k := range keys {
		av, aexists := am[k]
		bv, bexists := bm[k]
		if !aexists || !bexists {
			diff = append(diff, path+"."+k)
			continue
		}
		diff = append(diff, schemaDifferences(av, bv, path+"."+k)...)
	}
	slices.Sort(diff)
	return diff
}

func validateRoundTripExpectations(entry conversionCase) error {
	hasExpected := len(entry.ExpectedV2RoundTrip) != 0 || len(entry.ExpectedV3RoundTrip) != 0
	hasRationale := strings.TrimSpace(entry.NormalizationRationale) != ""
	if hasExpected && !hasRationale {
		return fmt.Errorf("conversion case %q has a round-trip exception without a normalization rationale", entry.CaseID)
	}
	if hasRationale && !hasExpected {
		return fmt.Errorf("conversion case %q has a normalization rationale without an explicit round-trip expectation", entry.CaseID)
	}
	for _, expected := range []json.RawMessage{entry.ExpectedV2RoundTrip, entry.ExpectedV3RoundTrip} {
		if len(expected) == 0 {
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(expected, &object); err != nil {
			return fmt.Errorf("conversion case %q has an invalid round-trip object: %w", entry.CaseID, err)
		}
		if object == nil {
			return fmt.Errorf("conversion case %q has a null round-trip object", entry.CaseID)
		}
	}
	return nil
}

func validateCases(entries []conversionCase, executed map[string]map[string]bool) error {
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.CaseID == "" || entry.V2Path == "" || entry.V3Path == "" || len(entry.OpenAPIPaths) == 0 || strings.TrimSpace(entry.ChangedSemantics) == "" {
			return fmt.Errorf("conversion case %q lacks identity, path, or changed semantics", entry.CaseID)
		}
		if len(entry.V2Object) == 0 || len(entry.V3Object) == 0 || len(entry.ExpectedV2) == 0 || len(entry.ExpectedV3) == 0 {
			return fmt.Errorf("conversion case %q lacks an executable fixture", entry.CaseID)
		}
		if err := validateRoundTripExpectations(entry); err != nil {
			return err
		}
		if seen[entry.CaseID] {
			return fmt.Errorf("duplicate conversion case %q", entry.CaseID)
		}
		seen[entry.CaseID] = true
		for _, category := range requiredCases {
			if !executed[entry.CaseID][category] {
				return fmt.Errorf("conversion case %q did not execute %s", entry.CaseID, category)
			}
		}
	}
	return nil
}

func conversionWire(t *testing.T, obj any) map[string]any {
	t.Helper()
	g := NewWithT(t)
	data, err := json.Marshal(obj)
	g.Expect(err).NotTo(HaveOccurred())
	var wire map[string]any
	g.Expect(json.Unmarshal(data, &wire)).To(Succeed())
	delete(wire, "apiVersion")
	delete(wire, "kind")
	return wire
}

const maasConversionAnnotation = "conversion.opendatahub.io/maas-v2-state"

func roundTripFixture(source, expected json.RawMessage) json.RawMessage {
	if len(expected) != 0 {
		return expected
	}
	return source
}

func expectedOpenAPIPaths(entries []conversionCase) []string {
	var paths []string
	for _, entry := range entries {
		paths = append(paths, entry.OpenAPIPaths...)
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func runConversionCases(t *testing.T, entries []conversionCase) map[string]map[string]bool {
	t.Helper()
	executed := make(map[string]map[string]bool, len(entries))
	for _, entry := range entries {
		t.Run(entry.CaseID, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(validateRoundTripExpectations(entry)).To(Succeed())
			var v2 dscv2.DataScienceCluster
			var v3 dscv3.DataScienceCluster
			var expectedV2 dscv2.DataScienceCluster
			var expectedV3 dscv3.DataScienceCluster
			var expectedV2RoundTrip dscv2.DataScienceCluster
			var expectedV3RoundTrip dscv3.DataScienceCluster
			g.Expect(json.Unmarshal(entry.V2Object, &v2)).To(Succeed())
			g.Expect(json.Unmarshal(entry.V3Object, &v3)).To(Succeed())
			g.Expect(json.Unmarshal(entry.ExpectedV2, &expectedV2)).To(Succeed())
			g.Expect(json.Unmarshal(entry.ExpectedV3, &expectedV3)).To(Succeed())
			g.Expect(json.Unmarshal(roundTripFixture(entry.V2Object, entry.ExpectedV2RoundTrip), &expectedV2RoundTrip)).To(Succeed())
			g.Expect(json.Unmarshal(roundTripFixture(entry.V3Object, entry.ExpectedV3RoundTrip), &expectedV3RoundTrip)).To(Succeed())
			executed[entry.CaseID] = make(map[string]bool, len(requiredCases))

			fromV2 := &dscv3.DataScienceCluster{}
			g.Expect(v2.ConvertTo(fromV2)).To(Succeed())
			forwardExpected := conversionWire(t, &expectedV3)
			g.Expect(conversionWire(t, fromV2)).To(Equal(forwardExpected), "forward, including the exact provenance marker")
			executed[entry.CaseID]["forward"] = true

			fromV3 := &dscv2.DataScienceCluster{}
			g.Expect(fromV3.ConvertFrom(&v3)).To(Succeed())
			g.Expect(conversionWire(t, fromV3)).To(Equal(conversionWire(t, &expectedV2)))
			executed[entry.CaseID]["backward"] = true

			v2RoundTrip := &dscv2.DataScienceCluster{}
			g.Expect(v2RoundTrip.ConvertFrom(fromV2)).To(Succeed())
			g.Expect(conversionWire(t, v2RoundTrip)).To(Equal(conversionWire(t, &expectedV2RoundTrip)), "v2 round trip: %s", entry.NormalizationRationale)
			executed[entry.CaseID]["v2-round-trip"] = true

			v3RoundTrip := &dscv3.DataScienceCluster{}
			g.Expect(fromV3.ConvertTo(v3RoundTrip)).To(Succeed())
			roundTripExpected := conversionWire(t, &expectedV3RoundTrip)
			g.Expect(conversionWire(t, v3RoundTrip)).To(Equal(roundTripExpected), "v3 round trip: %s", entry.NormalizationRationale)
			executed[entry.CaseID]["v3-round-trip"] = true
		})
	}
	return executed
}

func TestV2V3OpenAPIIdentical(t *testing.T) {
	g := NewWithT(t)
	root := repoRoot(t)
	entries := readConversionCases(t)
	g.Expect(validateCases(entries, runConversionCases(t, entries))).To(Succeed())
	expectedPaths := expectedOpenAPIPaths(entries)
	data, err := os.ReadFile(filepath.Join(root, "pkg/dsc/compare/testdata/v2-openapi-baseline.json"))
	g.Expect(err).NotTo(HaveOccurred())
	var expected baseline
	g.Expect(json.Unmarshal(data, &expected)).To(Succeed())
	for _, platform := range []struct{ name, path string }{
		{"OpenDataHub", "config/crd/bases/datasciencecluster.opendatahub.io_datascienceclusters.yaml"},
		{"rhoai", "config/rhoai/crd/bases/datasciencecluster.opendatahub.io_datascienceclusters.yaml"},
	} {
		t.Run(platform.name, func(t *testing.T) {
			g := NewWithT(t)
			path := filepath.Join(root, platform.path)
			v2 := readSchema(t, path, "v2")
			v3 := readSchema(t, path, "v3")
			g.Expect(fmt.Sprintf("%x", sha256.Sum256(canonical(t, v2)))).To(Equal(expected[platform.name]), "v2 baseline changed")
			g.Expect(schemaDifferences(v2, v3, "$")).To(Equal(expectedPaths), "unlisted v2/v3 OpenAPI difference")
		})
	}
}

func TestGeneratedDSCServingAndConversion(t *testing.T) {
	root := repoRoot(t)
	const warning = "datasciencecluster.opendatahub.io/v2 DataScienceCluster is deprecated; use datasciencecluster.opendatahub.io/v3 DataScienceCluster"
	for _, prefix := range []string{"config", "config/rhoai"} {
		t.Run(prefix, func(t *testing.T) {
			g := NewWithT(t)
			crdPath := filepath.Join(root, prefix, "crd/bases/datasciencecluster.opendatahub.io_datascienceclusters.yaml")
			data, err := os.ReadFile(crdPath)
			g.Expect(err).NotTo(HaveOccurred())
			var crd map[string]any
			g.Expect(yaml.Unmarshal(data, &crd)).To(Succeed())
			versions := crd["spec"].(map[string]any)["versions"].([]any)
			g.Expect(versions).To(HaveLen(2), "fresh installs must not serve v1")
			v2 := versions[0].(map[string]any)
			v3 := versions[1].(map[string]any)
			g.Expect(v2["name"]).To(Equal("v2"))
			g.Expect(v2["served"]).To(BeTrue())
			g.Expect(v2["storage"]).To(BeFalse())
			g.Expect(v2["deprecated"]).To(BeTrue())
			g.Expect(v2["deprecationWarning"]).To(Equal(warning))
			g.Expect(v3["name"]).To(Equal("v3"))
			g.Expect(v3["served"]).To(BeTrue())
			g.Expect(v3["storage"]).To(BeTrue())
			g.Expect(v3).NotTo(HaveKey("deprecated"))

			patchPath := filepath.Join(root, prefix, "crd/patches/webhook_in_datasciencecluster_datascienceclusters.yaml")
			patchBytes, err := os.ReadFile(patchPath)
			g.Expect(err).NotTo(HaveOccurred())
			var patch map[string]any
			g.Expect(yaml.Unmarshal(patchBytes, &patch)).To(Succeed())
			webhook := patch["spec"].(map[string]any)["conversion"].(map[string]any)["webhook"].(map[string]any)
			g.Expect(webhook["conversionReviewVersions"]).To(Equal([]any{"v1"}))
			service := webhook["clientConfig"].(map[string]any)["service"].(map[string]any)
			g.Expect(service["path"]).To(Equal("/convert"))

			webhookManifest, err := os.ReadFile(filepath.Join(root, prefix, "webhook/manifests.yaml"))
			g.Expect(err).NotTo(HaveOccurred())
			for _, suffix := range []string{"v2", "v3"} {
				g.Expect(string(webhookManifest)).To(ContainSubstring("/validate-datasciencecluster-" + suffix))
				g.Expect(string(webhookManifest)).To(ContainSubstring("/mutate-datasciencecluster-" + suffix))
			}
			g.Expect(strings.Contains(string(webhookManifest), "datasciencecluster-v1")).To(BeFalse(), "v1 admission must be absent")
		})
	}
}

func TestOpenAPIDifferenceDetection(t *testing.T) {
	for _, field := range []string{"type", "required", "nullable", "default", "enum", "x-kubernetes-validations", "x-kubernetes-list-type", "x-kubernetes-map-type", "properties"} {
		t.Run(field, func(t *testing.T) {
			g := NewWithT(t)
			a := map[string]any{field: "old"}
			b := map[string]any{field: "new"}
			g.Expect(schemaDifferences(a, b, "$")).To(Equal([]string{"$." + field}))
		})
	}
}

func readConversionCases(t *testing.T) []conversionCase {
	t.Helper()
	g := NewWithT(t)
	data, err := os.ReadFile(filepath.Join("testdata", "conversion-differences.json"))
	g.Expect(err).NotTo(HaveOccurred())
	var entries []conversionCase
	g.Expect(json.Unmarshal(data, &entries)).To(Succeed())
	return entries
}

func TestConversionRegistry(t *testing.T) {
	g := NewWithT(t)
	entries := readConversionCases(t)
	g.Expect(validateCases(entries, runConversionCases(t, entries))).To(Succeed())
	fixture := json.RawMessage(`{"metadata":{"name":"example"}}`)
	for _, category := range requiredCases {
		t.Run(category, func(t *testing.T) {
			g := NewWithT(t)
			entry := conversionCase{CaseID: "one", V2Path: "$.spec.old", V3Path: "$.spec.new", OpenAPIPaths: []string{"$.properties.spec"}, ChangedSemantics: "renamed", V2Object: fixture, V3Object: fixture, ExpectedV2: fixture, ExpectedV3: fixture}
			executed := map[string]map[string]bool{"one": {}}
			for _, other := range requiredCases {
				if other != category {
					executed["one"][other] = true
				}
			}
			g.Expect(validateCases([]conversionCase{entry}, executed)).To(MatchError(ContainSubstring(category)))
		})
	}
	// A semantic difference cannot be registered with an empty path or explanation.
	g.Expect(validateCases([]conversionCase{{CaseID: "empty"}}, nil)).To(HaveOccurred())
	g.Expect(validateCases([]conversionCase{{CaseID: "missing-fixture", V2Path: "a", V3Path: "b", OpenAPIPaths: []string{"$.properties.spec"}, ChangedSemantics: "added"}}, nil)).To(MatchError(ContainSubstring("executable fixture")))
}

func TestConversionRegistryRejectsUnexplainedNormalization(t *testing.T) {
	source := json.RawMessage(`{"spec":{"components":{"kserve":{"managementState":"Managed"},"aigateway":{"modelsAsAService":{}}}}}`)
	normalized := json.RawMessage(`{"spec":{"components":{"kserve":{"managementState":"Managed"},"aigateway":{"modelsAsAService":{"managementState":"Removed"}}}}}`)
	for _, version := range []string{"v2", "v3"} {
		t.Run(version, func(t *testing.T) {
			g := NewWithT(t)
			entry := conversionCase{
				CaseID: "unexplained", V2Path: "old", V3Path: "new",
				OpenAPIPaths: []string{"$.properties.spec"}, ChangedSemantics: "MaaS migration",
				V2Object: source, V3Object: source, ExpectedV2: normalized, ExpectedV3: normalized,
			}
			executed := map[string]map[string]bool{"unexplained": {}}
			for _, category := range requiredCases {
				executed[entry.CaseID][category] = true
			}
			// Direct conversion expectations must never silently authorize round-trip loss.
			g.Expect(roundTripFixture(source, nil)).To(Equal(source))
			g.Expect(roundTripFixture(source, nil)).NotTo(Equal(normalized))
			if version == "v2" {
				entry.ExpectedV2RoundTrip = normalized
			} else {
				entry.ExpectedV3RoundTrip = normalized
			}
			for _, rationale := range []string{"", " \n\t "} {
				entry.NormalizationRationale = rationale
				g.Expect(validateCases([]conversionCase{entry}, executed)).To(MatchError(ContainSubstring("without a normalization rationale")))
			}
			entry.NormalizationRationale = "Native v3 empty MaaS with managed KServe selects reverse-conversion legacy Removed on the forward leg."
			g.Expect(validateCases([]conversionCase{entry}, executed)).To(Succeed())
			g.Expect(roundTripFixture(source, normalized)).To(Equal(normalized))
			entry.ExpectedV2RoundTrip = nil
			entry.ExpectedV3RoundTrip = nil
			g.Expect(validateCases([]conversionCase{entry}, executed)).To(MatchError(ContainSubstring("without an explicit round-trip expectation")))
			entry.ExpectedV3RoundTrip = json.RawMessage(`null`)
			g.Expect(validateCases([]conversionCase{entry}, executed)).To(MatchError(ContainSubstring("null round-trip object")))
		})
	}
}

func TestConversionRegistryDeduplicatesOpenAPIPaths(t *testing.T) {
	g := NewWithT(t)
	entries := []conversionCase{
		{OpenAPIPaths: []string{"$.b", "$.a"}},
		{OpenAPIPaths: []string{"$.a", "$.b", "$.c"}},
	}
	g.Expect(expectedOpenAPIPaths(entries)).To(Equal([]string{"$.a", "$.b", "$.c"}))
	g.Expect(entries[0].OpenAPIPaths).To(Equal([]string{"$.b", "$.a"}))
}

func TestConversionRegistryRejectsUnknownMaaSMarker(t *testing.T) {
	for _, value := range []string{"", "Managed", "removed", "legacy-managed ", `{"legacy":"Managed"}`} {
		t.Run(value, func(t *testing.T) {
			g := NewWithT(t)
			source := &dscv3.DataScienceCluster{}
			source.Annotations = map[string]string{maasConversionAnnotation: value}
			target := &dscv2.DataScienceCluster{}
			g.Expect(target.ConvertFrom(source)).NotTo(Succeed(), "only the literal legacy-managed is valid")
			g.Expect(source.Annotations).To(Equal(map[string]string{maasConversionAnnotation: value}), "reject without mutating the source")
		})
	}
}
