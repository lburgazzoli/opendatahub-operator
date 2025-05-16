//nolint:ireturn
package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Funcs struct {
	Get                 func(client.Client, context.Context, client.ObjectKey, client.Object, ...client.GetOption) error
	List                func(client.Client, context.Context, client.ObjectList, ...client.ListOption) error
	Create              func(client.Client, context.Context, client.Object, ...client.CreateOption) error
	Delete              func(client.Client, context.Context, client.Object, ...client.DeleteOption) error
	Update              func(client.Client, context.Context, client.Object, ...client.UpdateOption) error
	Patch               func(client.Client, context.Context, client.Object, client.Patch, ...client.PatchOption) error
	DeleteAllOf         func(client.Client, context.Context, client.Object, ...client.DeleteAllOfOption) error
	Status              func(client.Client, context.Context, client.Object, ...client.SubResourceUpdateOption) error
	SubResource         func(client.Client, string) client.SubResourceClient
	Scheme              func(client.Client) *runtime.Scheme
	RESTMapper          func(client.Client) meta.RESTMapper
	GroupVersionKindFor func(client.Client, runtime.Object) (schema.GroupVersionKind, error)
	IsObjectNamespaced  func(client.Client, runtime.Object) (bool, error)
}

var _ client.Client = (*Client)(nil)

type Client struct {
	client client.Client
	funcs  Funcs
}

func New(client client.Client) *Client {
	return &Client{
		client: client,
	}
}

func (c *Client) WithFuncs(f Funcs) *Client {
	c.funcs = f
	return c
}

func (c *Client) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.funcs.Get != nil {
		return c.funcs.Get(c.client, ctx, key, obj, opts...)
	}
	return c.client.Get(ctx, key, obj, opts...)
}

func (c *Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.funcs.List != nil {
		return c.funcs.List(c.client, ctx, list, opts...)
	}
	return c.client.List(ctx, list, opts...)
}

func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.funcs.Create != nil {
		return c.funcs.Create(c.client, ctx, obj, opts...)
	}
	return c.client.Create(ctx, obj, opts...)
}

func (c *Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if c.funcs.Delete != nil {
		return c.funcs.Delete(c.client, ctx, obj, opts...)
	}
	return c.client.Delete(ctx, obj, opts...)
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.funcs.Update != nil {
		return c.funcs.Update(c.client, ctx, obj, opts...)
	}
	return c.client.Update(ctx, obj, opts...)
}

func (c *Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if c.funcs.Patch != nil {
		return c.funcs.Patch(c.client, ctx, obj, patch, opts...)
	}
	return c.client.Patch(ctx, obj, patch, opts...)
}

func (c *Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if c.funcs.DeleteAllOf != nil {
		return c.funcs.DeleteAllOf(c.client, ctx, obj, opts...)
	}
	return c.client.DeleteAllOf(ctx, obj, opts...)
}

func (c *Client) Status() client.SubResourceWriter {
	if c.funcs.Status != nil {
		return &subResourceWriter{client: c, statusFunc: c.funcs.Status}
	}
	return c.client.Status()
}

func (c *Client) SubResource(subResource string) client.SubResourceClient {
	if c.funcs.SubResource != nil {
		return c.funcs.SubResource(c.client, subResource)
	}
	return c.client.SubResource(subResource)
}

func (c *Client) Scheme() *runtime.Scheme {
	if c.funcs.Scheme != nil {
		return c.funcs.Scheme(c.client)
	}
	return c.client.Scheme()
}

func (c *Client) RESTMapper() meta.RESTMapper {
	if c.funcs.RESTMapper != nil {
		return c.funcs.RESTMapper(c.client)
	}
	return c.client.RESTMapper()
}

func (c *Client) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	if c.funcs.GroupVersionKindFor != nil {
		return c.funcs.GroupVersionKindFor(c.client, obj)
	}
	return c.client.GroupVersionKindFor(obj)
}

func (c *Client) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	if c.funcs.IsObjectNamespaced != nil {
		return c.funcs.IsObjectNamespaced(c.client, obj)
	}
	return c.client.IsObjectNamespaced(obj)
}

type subResourceWriter struct {
	client     *Client
	statusFunc func(client.Client, context.Context, client.Object, ...client.SubResourceUpdateOption) error
}

func (w *subResourceWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return w.client.client.Status().Create(ctx, obj, subResource, opts...)
}

func (w *subResourceWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return w.statusFunc(w.client.client, ctx, obj, opts...)
}

func (w *subResourceWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return w.client.client.Status().Patch(ctx, obj, patch, opts...)
}
