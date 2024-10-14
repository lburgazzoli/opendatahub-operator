package kustomize

import (
	"github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"
	"github.com/rs/xid"
	"path"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"testing"

	. "github.com/onsi/gomega"
)

const testEngineKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
resources:
- test-engine-cm.yaml
`

const testEngineConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-engine-cm
data:
  foo: bar
`

func TestEngine(t *testing.T) {
	g := NewWithT(t)
	id := xid.New().String()
	ns := xid.New().String()

	e := NewEngine()
	e.fs = filesys.MakeFsInMemory()

	_ = e.fs.MkdirAll(path.Join(id, defaultKustomizationFilePath))
	_ = e.fs.WriteFile(path.Join(id, defaultKustomizationFileName), []byte(testEngineKustomization))
	_ = e.fs.WriteFile(path.Join(id, "test-engine-cm.yaml"), []byte(testEngineConfigMap))

	r, err := e.Render(
		id,
		WithNamespace(ns),
		WithLabel("component.opendatahub.io/name", "foo"),
		WithLabel("platform.opendatahub.io/namespace", ns),
		WithAnnotations(map[string]string{
			"platform.opendatahub.io/release": "1.2.3",
			"platform.opendatahub.io/type":    "managed",
		}),
	)

	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(r).Should(And(
		HaveLen(1),
		HaveEach(And(
			jq.Match(`.metadata.namespace == "%s"`, ns),
			jq.Match(`.metadata.labels."component.opendatahub.io/name" == "%s"`, "foo"),
			jq.Match(`.metadata.labels."platform.opendatahub.io/namespace" == "%s"`, ns),
			jq.Match(`.metadata.annotations."platform.opendatahub.io/release" == "%s"`, "1.2.3"),
			jq.Match(`.metadata.annotations."platform.opendatahub.io/type" == "%s"`, "managed"),
		)),
	))

}
