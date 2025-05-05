package certconfigmapgenerator_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/certconfigmapgenerator"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"

	. "github.com/onsi/gomega"
)

func TestIsReservedNamespace(t *testing.T) {
	tests := []struct {
		name   string
		nsName string
		match  OmegaMatcher
	}{
		{
			name:   "should return true for openshift- prefix",
			nsName: "openshift-test",
			match:  BeTrue(),
		},
		{
			name:   "should return true for kube- prefix",
			nsName: "kube-system",
			match:  BeTrue(),
		},
		{
			name:   "should return true for default namespace",
			nsName: "default",
			match:  BeTrue(),
		},
		{
			name:   "should return true for openshift namespace",
			nsName: "openshift",
			match:  BeTrue(),
		},
		{
			name:   "should return false for non-reserved namespace",
			nsName: "my-namespace",
			match:  BeFalse(),
		},
		{
			name:   "should return false for empty namespace name",
			nsName: "",
			match:  BeFalse(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ns := resources.GvkToUnstructured(gvk.Namespace)
			ns.SetName(tt.nsName)

			g.Expect(
				certconfigmapgenerator.IsReservedNamespace(ns),
			).To(
				tt.match,
			)
		})
	}
}

func TestIsActiveNamespace(t *testing.T) {
	tests := []struct {
		name   string
		nsName string
		phase  corev1.NamespacePhase
		match  OmegaMatcher
	}{
		{
			name:   "should return true for active namespace",
			nsName: "active-ns",
			phase:  corev1.NamespaceActive,
			match:  BeTrue(),
		},
		{
			name:   "should return false for terminating namespace",
			nsName: "terminating-ns",
			phase:  corev1.NamespaceTerminating,
			match:  BeFalse(),
		},
		{
			name:   "should return false for non-existent phase",
			nsName: "no-phase-ns",
			phase:  "",
			match:  BeFalse(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			ns := resources.GvkToUnstructured(gvk.Namespace)
			ns.SetName(tt.nsName)

			if tt.phase != "" {
				err := unstructured.SetNestedField(ns.Object, string(tt.phase), "status", "phase")
				g.Expect(err).ShouldNot(HaveOccurred())
			}

			g.Expect(
				certconfigmapgenerator.IsActiveNamespace(ns),
			).To(
				tt.match,
			)
		})
	}
}
