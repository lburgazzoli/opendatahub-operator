package resources

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/davecgh/go-spew/spew"
	routev1 "github.com/openshift/api/route/v1"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
)

func ToUnstructured(obj any) (*unstructured.Unstructured, error) {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("unable to convert object %T to unstructured: %w", obj, err)
	}

	u := unstructured.Unstructured{
		Object: data,
	}

	return &u, nil
}

func Decode(decoder runtime.Decoder, content []byte) ([]unstructured.Unstructured, error) {
	results := make([]unstructured.Unstructured, 0)

	r := bytes.NewReader(content)
	yd := yaml.NewDecoder(r)

	for {
		var out map[string]interface{}

		err := yd.Decode(&out)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("unable to decode resource: %w", err)
		}

		if len(out) == 0 {
			continue
		}

		if out["Kind"] == "" {
			continue
		}

		encoded, err := yaml.Marshal(out)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal resource: %w", err)
		}

		var obj unstructured.Unstructured

		if _, _, err = decoder.Decode(encoded, nil, &obj); err != nil {
			if runtime.IsMissingKind(err) {
				continue
			}

			return nil, fmt.Errorf("unable to decode resource: %w", err)
		}

		results = append(results, obj)
	}

	return results, nil
}

func GvkToUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	return &u
}

func GvkToPartial(gvk schema.GroupVersionKind) *metav1.PartialObjectMetadata {
	obj := metav1.PartialObjectMetadata{}
	obj.SetGroupVersionKind(gvk)

	return &obj
}

func ObjToPartial(s *runtime.Scheme, in client.Object) (*metav1.PartialObjectMetadata, error) {
	if p, ok := in.(*metav1.PartialObjectMetadata); ok {
		return p, nil
	}

	gvk, err := GetGroupVersionKindForObject(s, in)
	if err != nil {
		return nil, err
	}

	obj := metav1.PartialObjectMetadata{}
	obj.SetGroupVersionKind(gvk)

	return &obj, nil
}

func IngressHost(r routev1.Route) string {
	if len(r.Status.Ingress) != 1 {
		return ""
	}

	in := r.Status.Ingress[0]

	for i := range in.Conditions {
		if in.Conditions[i].Type == routev1.RouteAdmitted && in.Conditions[i].Status == corev1.ConditionTrue {
			return in.Host
		}
	}

	return ""
}

func HasLabel(obj client.Object, k string, values ...string) bool {
	if obj == nil {
		return false
	}

	target := obj.GetLabels()
	if target == nil {
		return false
	}

	val, found := target[k]
	if !found {
		return false
	}

	return slices.Contains(values, val)
}

func SetLabels(obj client.Object, values map[string]string) {
	target := obj.GetLabels()
	if target == nil {
		target = make(map[string]string)
	}

	for k, v := range values {
		target[k] = v
	}

	obj.SetLabels(target)
}

func SetLabel(obj client.Object, k string, v string) string {
	target := obj.GetLabels()
	if target == nil {
		target = make(map[string]string)
	}

	old := target[k]
	target[k] = v

	obj.SetLabels(target)

	return old
}

func RemoveLabel(obj client.Object, k string) {
	target := obj.GetLabels()
	if target == nil {
		return
	}

	delete(target, k)

	obj.SetLabels(target)
}

func GetLabel(obj client.Object, k string) string {
	target := obj.GetLabels()
	if target == nil {
		return ""
	}

	return target[k]
}

func HasAnnotation(obj client.Object, k string, values ...string) bool {
	if obj == nil {
		return false
	}

	target := obj.GetAnnotations()
	if target == nil {
		return false
	}

	val, found := target[k]
	if !found {
		return false
	}

	return slices.Contains(values, val)
}

func SetAnnotations(obj client.Object, values map[string]string) {
	target := obj.GetAnnotations()
	if target == nil {
		target = make(map[string]string)
	}

	for k, v := range values {
		target[k] = v
	}

	obj.SetAnnotations(target)
}

func SetAnnotation(obj client.Object, k string, v string) string {
	target := obj.GetAnnotations()
	if target == nil {
		target = make(map[string]string)
	}

	old := target[k]
	target[k] = v

	obj.SetAnnotations(target)

	return old
}

func RemoveAnnotation(obj client.Object, k string) {
	target := obj.GetAnnotations()
	if target == nil {
		return
	}

	delete(target, k)

	obj.SetAnnotations(target)
}

func GetAnnotation(obj client.Object, k string) string {
	target := obj.GetAnnotations()
	if target == nil {
		return ""
	}

	return target[k]
}

// Hash generates an SHA-256 hash of an unstructured Kubernetes object, omitting
// specific fields that are typically irrelevant for hash comparison such as
// "creationTimestamp", "deletionTimestamp", "managedFields", "ownerReferences",
// "uid", "resourceVersion", and "status". It returns the computed hash as a byte
// slice or an error if the hashing process fails.
func Hash(in *unstructured.Unstructured) ([]byte, error) {
	obj := in.DeepCopy()
	unstructured.RemoveNestedField(obj.Object, "metadata", "uid")
	unstructured.RemoveNestedField(obj.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj.Object, "metadata", "deletionTimestamp")
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "metadata", "ownerReferences")
	unstructured.RemoveNestedField(obj.Object, "status")

	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}

	hasher := sha256.New()

	if _, err := printer.Fprintf(hasher, "%#v", obj); err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hasher.Sum(nil), nil
}

func EncodeToString(in []byte) string {
	return "v" + base64.RawURLEncoding.EncodeToString(in)
}

func KindForObject(scheme *runtime.Scheme, obj runtime.Object) (string, error) {
	if obj.GetObjectKind().GroupVersionKind().Kind != "" {
		return obj.GetObjectKind().GroupVersionKind().Kind, nil
	}

	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return "", fmt.Errorf("failed to get GVK: %w", err)
	}

	return gvk.Kind, nil
}

func GetGroupVersionKindForObject(s *runtime.Scheme, obj runtime.Object) (schema.GroupVersionKind, error) {
	if obj == nil {
		return schema.GroupVersionKind{}, errors.New("nil object")
	}

	if obj.GetObjectKind().GroupVersionKind().Version != "" && obj.GetObjectKind().GroupVersionKind().Kind != "" {
		return obj.GetObjectKind().GroupVersionKind(), nil
	}

	gvk, err := apiutil.GVKForObject(obj, s)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to get GVK: %w", err)
	}

	return gvk, nil
}

func EnsureGroupVersionKind(s *runtime.Scheme, obj client.Object) error {
	gvk, err := GetGroupVersionKindForObject(s, obj)
	if err != nil {
		return err
	}

	obj.GetObjectKind().SetGroupVersionKind(gvk)

	return nil
}

func HasDevFlags(in common.WithDevFlags) bool {
	if in == nil {
		return false
	}

	df := in.GetDevFlags()

	return df != nil && len(df.Manifests) != 0
}

// InstanceHasDevFlags checks if the given PlatformObject implements the WithDevFlags interface
// and if it has any DevFlags set. If the object does not implement WithDevFlags, it returns false.
// This function helps ensure that only objects with the WithDevFlags interface are processed for DevFlags.
func InstanceHasDevFlags(in common.PlatformObject) bool {
	if obj, ok := in.(common.WithDevFlags); ok {
		return HasDevFlags(obj)
	}
	return false
}

func NamespacedNameFromObject(obj client.Object) types.NamespacedName {
	return types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
}

// RemoveOwnerReferences removes all owner references from a Kubernetes object that match the provided predicate.
//
// This function iterates through the OwnerReferences of the given object, filters out those that satisfy
// the predicate, and updates the object in the cluster using the provided client.
//
// Parameters:
//   - ctx: The context for the request, which can carry deadlines, cancellation signals, and other request-scoped values.
//   - cli: A controller-runtime used to eventually retrieving the current Kubernetes object and update it.
//   - obj: The Kubernetes object whose OwnerReferences are to be filtered. It must implement client.Object.
//   - predicate: A function that takes an OwnerReference and returns true if the reference should be removed.
//
// Returns:
//   - An error if the update operation fails, otherwise nil.
func RemoveOwnerReferences(
	ctx context.Context,
	cli client.Client,
	ref client.Object,
	predicate func(reference metav1.OwnerReference) bool,
) error {
	oldRefs := ref.GetOwnerReferences()
	if len(oldRefs) == 0 {
		return nil
	}

	newRefs := oldRefs[:0]
	for _, ref := range oldRefs {
		if !predicate(ref) {
			newRefs = append(newRefs, ref)
		}
	}

	if len(newRefs) == len(oldRefs) {
		return nil
	}

	obj := ref
	convert := false

	// the kubernetes client does not support write operations related to
	// a PartialObjectMetadata, for such reason, we need to retrieve the
	// original object. Note that the retrieval may trigger a LIST+WATCH,
	// causing duplication in the cache, hence the client should not be
	// backed by a cache, to avoid excessive memory consumption.
	if _, ok := obj.(*metav1.PartialObjectMetadata); ok {
		obj = GvkToUnstructured(obj.GetObjectKind().GroupVersionKind())

		err := cli.Get(ctx, client.ObjectKeyFromObject(ref), obj)
		if err != nil {
			if k8serr.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf(
				"failed to retrieve object %s/%s with gvk %s to patch: %w",
				ref.GetNamespace(),
				ref.GetName(),
				ref.GetObjectKind().GroupVersionKind(),
				err,
			)
		}

		convert = true
	}

	obj.SetOwnerReferences(newRefs)

	// Update the object in the cluster
	if err := cli.Update(ctx, obj); err != nil {
		if k8serr.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf(
			"failed to remove owner references from object %s/%s with gvk %s: %w",
			obj.GetNamespace(),
			obj.GetName(),
			obj.GetObjectKind().GroupVersionKind(),
			err,
		)
	}

	if convert {
		ref.SetOwnerReferences(obj.GetOwnerReferences())
		ref.SetLabels(obj.GetLabels())
		ref.SetAnnotations(obj.GetAnnotations())
		ref.SetGeneration(obj.GetGeneration())
		ref.SetResourceVersion(obj.GetResourceVersion())
	}

	return nil
}

// IsOwnedByType checks if the given object (obj) is owned by an entity of the specified GroupVersionKind.
// It iterates through the object's owner references to determine ownership.
//
// Parameters:
// - obj: The Kubernetes object to check ownership for.
// - ownerGVK: The GroupVersionKind (GVK) of the expected owner.
//
// Returns:
// - A boolean indicating whether the object is owned by an entity of the specified GVK.
// - An error if any issue occurs while parsing the owner's API version.
func IsOwnedByType(obj client.Object, ownerGVK schema.GroupVersionKind) (bool, error) {
	for _, ref := range obj.GetOwnerReferences() {
		av, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return false, err
		}

		if av.Group == ownerGVK.Group && av.Version == ownerGVK.Version && ref.Kind == ownerGVK.Kind {
			return true, nil
		}
	}

	return false, nil
}
