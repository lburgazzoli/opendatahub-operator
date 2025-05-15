package manager

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

var _ client.Client = (*Client)(nil)

// Client wraps a client.Client to adapt Get and List operations based on the manager's configuration
type Client struct {
	client  client.Client
	manager *Manager
}

// NewClient returns a new Client that wraps the given client.Client
func NewClient(c client.Client, m *Manager) *Client {
	return &Client{
		client:  c,
		manager: m,
	}
}

func (c *Client) Get(
	ctx context.Context,
	key client.ObjectKey,
	out client.Object,
	opts ...client.GetOption,
) error {
	gvk := out.GetObjectKind().GroupVersionKind()

	switch {
	case c.manager.IsUnstructuredObject(gvk):
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)

		if err := c.client.Get(ctx, key, u, opts...); err != nil {
			return err
		}

		return resources.ObjectFromUnstructured(c.Scheme(), u, out)

	case c.manager.IsTypedObject(gvk):
		return c.client.Get(ctx, key, out, opts...)

	default:
		p := &metav1.PartialObjectMetadata{}
		p.SetGroupVersionKind(gvk)

		if err := c.client.Get(ctx, key, p, opts...); err != nil {
			return err
		}

		return resources.FromPartialObjectMetadata(p, out)
	}
}

func (c *Client) List(
	ctx context.Context,
	out client.ObjectList,
	opts ...client.ListOption,
) error {
	gvk, err := resources.GetGroupVersionKindForList(c.Scheme(), out)
	if err != nil {
		return err
	}

	switch {
	case c.manager.IsUnstructuredObject(gvk):
		l := unstructured.UnstructuredList{}
		l.SetGroupVersionKind(gvk)

		if err := c.client.List(ctx, &l, opts...); err != nil {
			return err
		}

		return resources.UnstructuredListToObjectList(c.Scheme(), l, out)

	case c.manager.IsTypedObject(gvk):
		return c.client.List(ctx, out, opts...)

	default:
		l := metav1.PartialObjectMetadataList{}
		l.SetGroupVersionKind(gvk)

		if err := c.client.List(ctx, &l, opts...); err != nil {
			return err
		}

		return resources.FromPartialObjectMetadataList(c.Scheme(), l, out)
	}
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return c.client.Create(ctx, obj, opts...)
}

func (c *Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return c.client.Delete(ctx, obj, opts...)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return c.client.Update(ctx, obj, opts...)
}

func (c *Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *Client) Status() client.StatusWriter {
	return c.client.Status()
}

func (c *Client) Scheme() *runtime.Scheme {
	return c.client.Scheme()
}

func (c *Client) RESTMapper() meta.RESTMapper {
	return c.client.RESTMapper()
}

func (c *Client) SubResource(subResource string) client.SubResourceClient {
	return c.client.SubResource(subResource)
}

func (c *Client) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return c.client.GroupVersionKindFor(obj)
}

func (c *Client) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return c.client.IsObjectNamespaced(obj)
}
