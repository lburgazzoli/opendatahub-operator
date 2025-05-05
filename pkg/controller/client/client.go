package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/metrics"
)

// Ensure the wrapper implements the Client interface.
var _ client.Client = &Client{}

type Client struct {
	delegate client.Client
}

// Wrap creates a new wrapper around the provided client.
func Wrap(c client.Client) *Client {
	return &Client{
		delegate: c,
	}
}

func (c *Client) Scheme() *runtime.Scheme {
	return c.delegate.Scheme()
}

func (c *Client) RESTMapper() meta.RESTMapper {
	return c.delegate.RESTMapper()
}

func (c *Client) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.delegate.GroupVersionKindFor(obj)
}

func (c *Client) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.delegate.IsObjectNamespaced(obj)
}

func (c *Client) SubResource(subResource string) client.SubResourceClient {
	return c.delegate.SubResource(subResource)
}

func (c *Client) Status() client.StatusWriter {
	return c.delegate.Status()
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return c.delegate.Create(ctx, obj, opts...)
}

func (c *Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return c.delegate.Delete(ctx, obj, opts...)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.delegate.Update(ctx, obj, opts...)
}

func (c *Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.delegate.Patch(ctx, obj, patch, opts...)
}

func (c *Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return c.delegate.DeleteAllOf(ctx, obj, opts...)
}

func (c *Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch obj.(type) {
	case *unstructured.Unstructured:
		// If it's already unstructured, use the underlying client directly
		return c.delegate.Get(ctx, key, obj, opts...)
	case *metav1.PartialObjectMetadata:
		// If it's partial, use the underlying client directly
		return c.delegate.Get(ctx, key, obj, opts...)
	}

	// Convert to unstructured for the request
	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	// Get the object as unstructured
	if err := c.delegate.Get(ctx, key, u, opts...); err != nil {
		return err
	}

	metrics.ConvertedResourcesTotal.WithLabelValues(
		gvk.GroupVersion().String(),
		gvk.Kind,
		"get",
	).Set(1)

	return c.Scheme().Convert(u, obj, ctx)
}

func (c *Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	switch list.(type) {
	case *unstructured.UnstructuredList:
		// If it's already unstructured, use the underlying client directly
		return c.delegate.List(ctx, list, opts...)
	case *metav1.PartialObjectMetadataList:
		// If it's partial, use the underlying client directly
		return c.delegate.List(ctx, list, opts...)
	}

	gvk, err := getItemGVKFromObjectList(c.Scheme(), list)
	if err != nil {
		return err
	}

	uList := unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(gvk)

	// List objects as unstructured
	if err := c.delegate.List(ctx, &uList, opts...); err != nil {
		return err
	}

	// Copy list metadata
	list.SetResourceVersion(uList.GetResourceVersion())
	list.SetContinue(uList.GetContinue())
	list.SetRemainingItemCount(uList.GetRemainingItemCount())

	if err := c.unstructuredListToObjectList(ctx, uList.Items, gvk, list); err != nil {
		return err
	}

	metrics.ConvertedResourcesTotal.WithLabelValues(
		gvk.GroupVersion().String(),
		gvk.Kind,
		"list",
	).Set(1)

	return nil
}
