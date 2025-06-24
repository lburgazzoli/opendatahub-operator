package cleanup_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/cleanup"
	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"

	. "github.com/onsi/gomega"
)

func TestDeleteHandler_ManagedAnnotationFilter(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	unmanagedObj := &unstructured.Unstructured{}
	unmanagedObj.SetName("test-configmap")
	unmanagedObj.SetNamespace("test-ns")
	unmanagedObj.SetGroupVersionKind(gvk.ConfigMap)
	unmanagedObj.SetAnnotations(map[string]string{
		annotations.ManagedByODHOperator: "false",
	})
	unmanagedObj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "test.io/v1",
		Kind:       "TestKind",
		Name:       "test-owner",
		UID:        "test-uid",
	}})

	cli, err := fakeclient.New(fakeclient.WithObjects(unmanagedObj))
	g.Expect(err).ShouldNot(HaveOccurred())

	mockRequest := &odhTypes.ReconciliationRequest{
		Kind:   schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "TestKind"},
		Client: cli,
	}

	handlerWithCheck := cleanup.DeleteHandler(cleanup.WithManagedAnnotationCheck(true))
	handlerWithoutCheck := cleanup.DeleteHandler(cleanup.WithManagedAnnotationCheck(false))

	processed, err := handlerWithCheck(ctx, mockRequest, *unmanagedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeFalse())

	retrievedObj := unstructured.Unstructured{}
	retrievedObj.SetGroupVersionKind(gvk.ConfigMap)
	err = cli.Get(ctx, client.ObjectKey{Name: "test-configmap", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).ShouldNot(HaveOccurred())

	processed, err = handlerWithoutCheck(ctx, mockRequest, *unmanagedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeTrue())

	err = cli.Get(ctx, client.ObjectKey{Name: "test-configmap", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).Should(HaveOccurred())
}

func TestDeleteHandler_CustomObjectPredicate(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	testObj := &unstructured.Unstructured{}
	testObj.SetName("test-object")
	testObj.SetNamespace("test-ns")
	testObj.SetGroupVersionKind(gvk.ConfigMap)
	testObj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "test.io/v1",
		Kind:       "TestKind",
		Name:       "test-owner",
		UID:        "test-uid",
	}})

	cli, err := fakeclient.New(fakeclient.WithObjects(testObj))
	g.Expect(err).ShouldNot(HaveOccurred())

	mockRequest := &odhTypes.ReconciliationRequest{
		Kind:   schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "TestKind"},
		Client: cli,
	}

	handlerWithReject := cleanup.DeleteHandler(cleanup.WithObjectPredicate(cleanup.NamesNotInObjectPredicate("test-object")))
	handlerWithAccept := cleanup.DeleteHandler(cleanup.WithObjectPredicate(cleanup.CatchAllObjectPredicate))

	processed, err := handlerWithReject(ctx, mockRequest, *testObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeFalse())

	retrievedObj := unstructured.Unstructured{}
	retrievedObj.SetGroupVersionKind(gvk.ConfigMap)
	err = cli.Get(ctx, client.ObjectKey{Name: "test-object", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).ShouldNot(HaveOccurred())

	processed, err = handlerWithAccept(ctx, mockRequest, *testObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeTrue())

	err = cli.Get(ctx, client.ObjectKey{Name: "test-object", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).Should(HaveOccurred())
}

func TestDeleteHandler_OwnershipCheck(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	ownedObj := &unstructured.Unstructured{}
	ownedObj.SetName("owned-object")
	ownedObj.SetNamespace("test-ns")
	ownedObj.SetGroupVersionKind(gvk.ConfigMap)
	ownedObj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "test.io/v1",
		Kind:       "TestKind",
		Name:       "test-owner",
		UID:        "test-uid",
	}})

	notOwnedObj := &unstructured.Unstructured{}
	notOwnedObj.SetName("not-owned-object")
	notOwnedObj.SetNamespace("test-ns")
	notOwnedObj.SetGroupVersionKind(gvk.ConfigMap)
	notOwnedObj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion: "test.io/v1",
		Kind:       "OtherKind",
		Name:       "other-owner",
		UID:        "other-uid",
	}})

	cli, err := fakeclient.New(fakeclient.WithObjects(ownedObj, notOwnedObj))
	g.Expect(err).ShouldNot(HaveOccurred())

	mockRequest := &odhTypes.ReconciliationRequest{
		Kind:   schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "TestKind"},
		Client: cli,
	}

	handlerWithOwnershipCheck := cleanup.DeleteHandler(
		cleanup.WithOwnershipCheck(true),
		cleanup.WithObjectPredicate(cleanup.CatchAllObjectPredicate),
	)
	handlerWithoutOwnershipCheck := cleanup.DeleteHandler(
		cleanup.WithOwnershipCheck(false),
		cleanup.WithObjectPredicate(cleanup.CatchAllObjectPredicate),
	)

	processed, err := handlerWithOwnershipCheck(ctx, mockRequest, *ownedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeTrue())

	retrievedObj := unstructured.Unstructured{}
	retrievedObj.SetGroupVersionKind(gvk.ConfigMap)
	err = cli.Get(ctx, client.ObjectKey{Name: "owned-object", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).Should(HaveOccurred())

	processed, err = handlerWithOwnershipCheck(ctx, mockRequest, *notOwnedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeFalse())

	err = cli.Get(ctx, client.ObjectKey{Name: "not-owned-object", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).ShouldNot(HaveOccurred())

	processed, err = handlerWithoutOwnershipCheck(ctx, mockRequest, *notOwnedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeTrue())

	err = cli.Get(ctx, client.ObjectKey{Name: "not-owned-object", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).Should(HaveOccurred())
}

func TestDeleteHandler_DefaultConfiguration(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	unmanagedObj := &unstructured.Unstructured{}
	unmanagedObj.SetName("test-configmap")
	unmanagedObj.SetNamespace("test-ns")
	unmanagedObj.SetGroupVersionKind(gvk.ConfigMap)
	unmanagedObj.SetAnnotations(map[string]string{
		annotations.ManagedByODHOperator: "false",
	})

	cli, err := fakeclient.New(fakeclient.WithObjects(unmanagedObj))
	g.Expect(err).ShouldNot(HaveOccurred())

	mockRequest := &odhTypes.ReconciliationRequest{
		Kind:   schema.GroupVersionKind{Group: "test.io", Version: "v1", Kind: "TestKind"},
		Client: cli,
	}

	defaultHandler := cleanup.DeleteHandler()

	processed, err := defaultHandler(ctx, mockRequest, *unmanagedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(processed).Should(BeFalse())

	retrievedObj := unstructured.Unstructured{}
	retrievedObj.SetGroupVersionKind(gvk.ConfigMap)
	err = cli.Get(ctx, client.ObjectKey{Name: "test-configmap", Namespace: "test-ns"}, &retrievedObj)
	g.Expect(err).ShouldNot(HaveOccurred())
}
