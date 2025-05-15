package manager

import (
	"context"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

var _ client.Client = (*Client)(nil)

type Client struct {
	client  client.Client
	manager *Manager
}

func NewClient(m *Manager, c client.Client) *Client {
	return &Client{
		manager: m,
		client:  c,
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

	case c.manager.IsTypedObject(gvk):
		return c.client.Get(ctx, key, out, opts...)

	case c.manager.IsUnstructuredObject(gvk):
		u, err := ToUnstructured(c.Scheme(), out)
		if err != nil {
			return err
		}

		if err := c.client.Get(ctx, key, u, opts...); err != nil {
			return err
		}

		return FromUnstructured(c.Scheme(), u, out)

	default:
		p, err := ToPartial(c.Scheme(), out)
		if err != nil {
			return err
		}

		if err := c.client.Get(ctx, key, p, opts...); err != nil {
			return err
		}

		return FromPartial(c.Scheme(), p, out)
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
	case c.manager.IsTypedObject(gvk):
		return c.client.List(ctx, out, opts...)

	case c.manager.IsUnstructuredObject(gvk):
		l, err := ToUnstructuredList(c.Scheme(), out)
		if err != nil {
			return err
		}

		if err := c.client.List(ctx, l, opts...); err != nil {
			return err
		}

		return FromUnstructuredList(c.Scheme(), *l, out)

	default:
		l, err := ToPartialList(c.Scheme(), out)
		if err != nil {
			return err
		}

		if err := c.client.List(ctx, l, opts...); err != nil {
			return err
		}

		return FromPartialList(c.Scheme(), *l, out)
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
