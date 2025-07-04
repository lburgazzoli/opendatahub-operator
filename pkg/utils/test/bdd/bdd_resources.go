package bdd

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResourceClient struct {
	client   client.Client
	resolver *ResourceResolver
}

func (c *ResourceClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	resourceType string,
	opts ...client.GetOption,
) (*unstructured.Unstructured, error) {
	if resourceType == "" {
		return nil, errors.New("resource type cannot be empty")
	}

	if key.Name == "" {
		return nil, errors.New("resource name cannot be empty")
	}

	resource, err := c.resolver.Resolve(resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type '%s': %w", resourceType, err)
	}

	if err := ValidateResourceScope(&resource.RESTMapping, key.Namespace != ""); err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resource.GroupVersionKind())

	if err := c.client.Get(ctx, key, obj, opts...); err != nil {
		return nil, err
	}

	return obj, nil
}

func (c *ResourceClient) List(
	ctx context.Context,
	namespace string,
	resourceType string,
	labelSelector labels.Selector,
	opts ...client.ListOption,
) (*unstructured.UnstructuredList, error) {
	if resourceType == "" {
		return nil, errors.New("resource type cannot be empty")
	}

	resource, err := c.resolver.Resolve(resourceType)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource type '%s': %w", resourceType, err)
	}

	if err := ValidateResourceScope(&resource.RESTMapping, namespace != ""); err != nil {
		return nil, err
	}

	listObj := &unstructured.UnstructuredList{}
	listObj.SetGroupVersionKind(resource.GroupVersionKind())

	listOpts := slices.Clone(opts)
	listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: labelSelector})
	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if err := c.client.List(ctx, listObj, listOpts...); err != nil {
		return nil, err
	}

	return listObj, nil
}

func (c *ResourceClient) Delete(
	ctx context.Context,
	key client.ObjectKey,
	resourceType string,
	opts ...client.DeleteOption,
) error {
	if resourceType == "" {
		return errors.New("resource type cannot be empty")
	}

	if key.Name == "" {
		return errors.New("resource name cannot be empty")
	}

	resource, err := c.resolver.Resolve(resourceType)
	if err != nil {
		return fmt.Errorf("failed to resolve resource type '%s': %w", resourceType, err)
	}

	if err := ValidateResourceScope(&resource.RESTMapping, key.Namespace != ""); err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(resource.GroupVersionKind())
	obj.SetName(key.Name)
	obj.SetNamespace(key.Namespace)

	if err := c.client.Delete(ctx, obj, opts...); err != nil {
		return err
	}

	return nil
}
