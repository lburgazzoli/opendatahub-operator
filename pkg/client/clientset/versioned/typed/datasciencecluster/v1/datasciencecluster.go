/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"context"
	json "encoding/json"
	"fmt"
	"time"

	v1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	datascienceclusterv1 "github.com/opendatahub-io/opendatahub-operator/v2/pkg/client/applyconfiguration/datasciencecluster/v1"
	scheme "github.com/opendatahub-io/opendatahub-operator/v2/pkg/client/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// DataScienceClustersGetter has a method to return a DataScienceClusterInterface.
// A group's client should implement this interface.
type DataScienceClustersGetter interface {
	DataScienceClusters() DataScienceClusterInterface
}

// DataScienceClusterInterface has methods to work with DataScienceCluster resources.
type DataScienceClusterInterface interface {
	Create(ctx context.Context, dataScienceCluster *v1.DataScienceCluster, opts metav1.CreateOptions) (*v1.DataScienceCluster, error)
	Update(ctx context.Context, dataScienceCluster *v1.DataScienceCluster, opts metav1.UpdateOptions) (*v1.DataScienceCluster, error)
	UpdateStatus(ctx context.Context, dataScienceCluster *v1.DataScienceCluster, opts metav1.UpdateOptions) (*v1.DataScienceCluster, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.DataScienceCluster, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.DataScienceClusterList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.DataScienceCluster, err error)
	Apply(ctx context.Context, dataScienceCluster *datascienceclusterv1.DataScienceClusterApplyConfiguration, opts metav1.ApplyOptions) (result *v1.DataScienceCluster, err error)
	ApplyStatus(ctx context.Context, dataScienceCluster *datascienceclusterv1.DataScienceClusterApplyConfiguration, opts metav1.ApplyOptions) (result *v1.DataScienceCluster, err error)
	DataScienceClusterExpansion
}

// dataScienceClusters implements DataScienceClusterInterface
type dataScienceClusters struct {
	client rest.Interface
}

// newDataScienceClusters returns a DataScienceClusters
func newDataScienceClusters(c *DatascienceclusterV1Client) *dataScienceClusters {
	return &dataScienceClusters{
		client: c.RESTClient(),
	}
}

// Get takes name of the dataScienceCluster, and returns the corresponding dataScienceCluster object, and an error if there is any.
func (c *dataScienceClusters) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.DataScienceCluster, err error) {
	result = &v1.DataScienceCluster{}
	err = c.client.Get().
		Resource("datascienceclusters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of DataScienceClusters that match those selectors.
func (c *dataScienceClusters) List(ctx context.Context, opts metav1.ListOptions) (result *v1.DataScienceClusterList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.DataScienceClusterList{}
	err = c.client.Get().
		Resource("datascienceclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested dataScienceClusters.
func (c *dataScienceClusters) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("datascienceclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a dataScienceCluster and creates it.  Returns the server's representation of the dataScienceCluster, and an error, if there is any.
func (c *dataScienceClusters) Create(ctx context.Context, dataScienceCluster *v1.DataScienceCluster, opts metav1.CreateOptions) (result *v1.DataScienceCluster, err error) {
	result = &v1.DataScienceCluster{}
	err = c.client.Post().
		Resource("datascienceclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(dataScienceCluster).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a dataScienceCluster and updates it. Returns the server's representation of the dataScienceCluster, and an error, if there is any.
func (c *dataScienceClusters) Update(ctx context.Context, dataScienceCluster *v1.DataScienceCluster, opts metav1.UpdateOptions) (result *v1.DataScienceCluster, err error) {
	result = &v1.DataScienceCluster{}
	err = c.client.Put().
		Resource("datascienceclusters").
		Name(dataScienceCluster.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(dataScienceCluster).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *dataScienceClusters) UpdateStatus(ctx context.Context, dataScienceCluster *v1.DataScienceCluster, opts metav1.UpdateOptions) (result *v1.DataScienceCluster, err error) {
	result = &v1.DataScienceCluster{}
	err = c.client.Put().
		Resource("datascienceclusters").
		Name(dataScienceCluster.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(dataScienceCluster).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the dataScienceCluster and deletes it. Returns an error if one occurs.
func (c *dataScienceClusters) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("datascienceclusters").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *dataScienceClusters) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("datascienceclusters").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched dataScienceCluster.
func (c *dataScienceClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.DataScienceCluster, err error) {
	result = &v1.DataScienceCluster{}
	err = c.client.Patch(pt).
		Resource("datascienceclusters").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

// Apply takes the given apply declarative configuration, applies it and returns the applied dataScienceCluster.
func (c *dataScienceClusters) Apply(ctx context.Context, dataScienceCluster *datascienceclusterv1.DataScienceClusterApplyConfiguration, opts metav1.ApplyOptions) (result *v1.DataScienceCluster, err error) {
	if dataScienceCluster == nil {
		return nil, fmt.Errorf("dataScienceCluster provided to Apply must not be nil")
	}
	patchOpts := opts.ToPatchOptions()
	data, err := json.Marshal(dataScienceCluster)
	if err != nil {
		return nil, err
	}
	name := dataScienceCluster.Name
	if name == nil {
		return nil, fmt.Errorf("dataScienceCluster.Name must be provided to Apply")
	}
	result = &v1.DataScienceCluster{}
	err = c.client.Patch(types.ApplyPatchType).
		Resource("datascienceclusters").
		Name(*name).
		VersionedParams(&patchOpts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *dataScienceClusters) ApplyStatus(ctx context.Context, dataScienceCluster *datascienceclusterv1.DataScienceClusterApplyConfiguration, opts metav1.ApplyOptions) (result *v1.DataScienceCluster, err error) {
	if dataScienceCluster == nil {
		return nil, fmt.Errorf("dataScienceCluster provided to Apply must not be nil")
	}
	patchOpts := opts.ToPatchOptions()
	data, err := json.Marshal(dataScienceCluster)
	if err != nil {
		return nil, err
	}

	name := dataScienceCluster.Name
	if name == nil {
		return nil, fmt.Errorf("dataScienceCluster.Name must be provided to Apply")
	}

	result = &v1.DataScienceCluster{}
	err = c.client.Patch(types.ApplyPatchType).
		Resource("datascienceclusters").
		Name(*name).
		SubResource("status").
		VersionedParams(&patchOpts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
