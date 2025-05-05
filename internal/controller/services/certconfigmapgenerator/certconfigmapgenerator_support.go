// Package trustedcabundle provides utility functions to create and check trusted CA bundle configmap from DSCI CRD
package certconfigmapgenerator

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

const (
	CAConfigMapName           = "odh-trusted-ca-bundle"
	CADataFieldName           = "odh-ca-bundle.crt"
	TrustedCABundleFieldOwner = resources.PlatformFieldOwner + "/trustedcabundle"
	PartOf                    = "opendatahub-operator"
	NSListLimit               = 500
)

// CreateOdhTrustedCABundleConfigMap creates a configMap 'odh-trusted-ca-bundle' in given namespace with labels and data
// or update existing odh-trusted-ca-bundle configmap if already exists with new content of .data.odh-ca-bundle.crt
// this is certificates for the cluster trusted CA Cert Bundle.
func CreateOdhTrustedCABundleConfigMap(ctx context.Context, cli client.Client, namespace string, customCAData string) error {
	// Adding newline breaker if user input does not have it
	customCAData = strings.TrimSpace(customCAData) + "\n"

	// Expected configmap for the given namespace
	desiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CAConfigMapName,
			Namespace: namespace,
			Labels: map[string]string{
				labels.K8SCommon.PartOf: PartOf,
				// Label 'config.openshift.io/inject-trusted-cabundle' required for the Cluster Network Operator(CNO)
				// to inject the cluster trusted CA bundle into .data["ca-bundle.crt"]
				labels.InjectTrustCA: labels.True,
			},
		},
		// Add the DSCInitialzation specified TrustedCABundle.CustomCABundle to CM's data.odh-ca-bundle.crt field
		//
		// Additionally, the CNO operator will automatically create and maintain ca-bundle.crt
		// if label 'config.openshift.io/inject-trusted-cabundle' is true
		Data: map[string]string{
			CADataFieldName: customCAData,
		},
	}

	err := resources.Apply(
		ctx,
		cli,
		desiredConfigMap,
		client.FieldOwner(TrustedCABundleFieldOwner),
		client.ForceOwnership,
	)

	if err != nil {
		return err
	}

	return nil
}

func DeleteOdhTrustedCABundleConfigMap(ctx context.Context, cli client.Client, namespace string) error {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CAConfigMapName,
			Namespace: namespace},
	}

	err := cli.Delete(ctx, &cm)
	if err != nil && !k8serr.IsNotFound(err) {
		return err
	}

	return nil
}

// dsciEventHandler creates an event handler for DSCInitialization events. When a DSCInitialization
// resource changes, this handler enqueues reconciliation requests for all namespaces in the cluster,
// allowing the controller to update CA Bundle configuration across all namespaces.
//
// Parameters:
//   - cli: Kubernetes client used to list namespaces
//
// Returns:
//   - handler.EventHandler: Event handler that maps DSCInitialization events to namespace reconcile requests
func dsciEventHandler(cli client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, _ client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0)

		lo := client.ListOptions{
			Limit: NSListLimit,
		}

		for {
			namespaces := unstructured.UnstructuredList{}
			namespaces.SetGroupVersionKind(gvk.Namespace)

			if err := cli.List(ctx, &namespaces, &lo); err != nil {
				return []reconcile.Request{}
			}

			for _, ns := range namespaces.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: resources.NamespacedNameFromObject(&ns),
				})
			}

			lo.Continue = namespaces.GetContinue()

			if lo.Continue == "" {
				break
			}
		}

		return requests
	})
}

func IsReservedNamespace(ns *unstructured.Unstructured) bool {
	switch {
	case strings.HasPrefix(ns.GetName(), "openshift-"):
		return true
	case strings.HasPrefix(ns.GetName(), "kube-"):
		return true
	case ns.GetName() == "default":
		return true
	case ns.GetName() == "openshift":
		return true
	default:
		return false
	}
}

func IsActiveNamespace(ns *unstructured.Unstructured) bool {
	phase, ok, err := unstructured.NestedString(ns.Object, "status", "phase")
	switch {
	case err != nil:
		return false
	case !ok:
		return false
	default:
		return phase == string(corev1.NamespaceActive)
	}
}
