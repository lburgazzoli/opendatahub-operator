package manager

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

func ToUnstructured(s *runtime.Scheme, in client.Object) (*unstructured.Unstructured, error) {
	if err := resources.EnsureGroupVersionKind(s, in); err != nil {
		return nil, err
	}

	if u, ok := in.(*unstructured.Unstructured); ok {
		return u, nil
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(in.GetObjectKind().GroupVersionKind())

	return u, nil
}

func FromUnstructured(s *runtime.Scheme, in *unstructured.Unstructured, out client.Object) error {
	if err := resources.EnsureGroupVersionKind(s, in); err != nil {
		return err
	}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(in.Object, out); err != nil {
		return fmt.Errorf("unable to convert unstructured object to %T: %w", out, err)
	}

	return nil
}

func ToUnstructuredList(_ *runtime.Scheme, in client.ObjectList) (*unstructured.UnstructuredList, error) {
	if u, ok := in.(*unstructured.UnstructuredList); ok {
		return u, nil
	}

	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(in.GetObjectKind().GroupVersionKind())

	return u, nil
}

func FromUnstructuredList(
	s *runtime.Scheme,
	in unstructured.UnstructuredList,
	out client.ObjectList,
) error {
	items := make([]runtime.Object, 0, len(in.Items))
	for _, u := range in.Items {
		obj, err := s.New(u.GetObjectKind().GroupVersionKind())
		if err != nil {
			return err
		}

		o, ok := obj.(client.Object)
		if !ok {
			return errors.New("not a client.Object")
		}

		if err := FromUnstructured(s, &u, o); err != nil {
			return err
		}

		items = append(items, obj)
	}

	return meta.SetList(out, items)
}

func ToPartial(s *runtime.Scheme, in client.Object) (*metav1.PartialObjectMetadata, error) {
	if err := resources.EnsureGroupVersionKind(s, in); err != nil {
		return nil, err
	}

	if p, ok := in.(*metav1.PartialObjectMetadata); ok {
		return p, nil
	}

	p := &metav1.PartialObjectMetadata{}
	p.SetGroupVersionKind(in.GetObjectKind().GroupVersionKind())

	return p, nil
}

func FromPartial(_ *runtime.Scheme, in *metav1.PartialObjectMetadata, out client.Object) error {
	if targetPartial, ok := out.(*metav1.PartialObjectMetadata); ok {
		*targetPartial = *in
		return nil
	}

	out.GetObjectKind().SetGroupVersionKind(in.GetObjectKind().GroupVersionKind())

	out.SetName(in.GetName())
	out.SetNamespace(in.GetNamespace())
	out.SetLabels(in.GetLabels())
	out.SetAnnotations(in.GetAnnotations())
	out.SetOwnerReferences(in.GetOwnerReferences())
	out.SetFinalizers(in.GetFinalizers())
	out.SetGeneration(in.GetGeneration())
	out.SetResourceVersion(in.GetResourceVersion())
	out.SetUID(in.GetUID())
	out.SetCreationTimestamp(in.GetCreationTimestamp())
	out.SetDeletionTimestamp(in.GetDeletionTimestamp())
	out.SetDeletionGracePeriodSeconds(in.GetDeletionGracePeriodSeconds())

	return nil
}

func ToPartialList(_ *runtime.Scheme, in client.ObjectList) (*metav1.PartialObjectMetadataList, error) {
	if p, ok := in.(*metav1.PartialObjectMetadataList); ok {
		return p, nil
	}

	p := &metav1.PartialObjectMetadataList{}
	p.SetGroupVersionKind(in.GetObjectKind().GroupVersionKind())

	return p, nil
}

func FromPartialList(
	s *runtime.Scheme,
	in metav1.PartialObjectMetadataList,
	out client.ObjectList,
) error {
	items := make([]runtime.Object, 0, len(in.Items))
	for _, u := range in.Items {
		obj, err := s.New(u.GetObjectKind().GroupVersionKind())
		if err != nil {
			return err
		}

		o, ok := obj.(client.Object)
		if !ok {
			return errors.New("not a client.Object")
		}

		if err := FromPartial(s, &u, o); err != nil {
			return err
		}

		items = append(items, obj)
	}

	return meta.SetList(out, items)
}
