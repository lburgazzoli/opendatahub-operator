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
// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	dscinitializationv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	versioned "github.com/opendatahub-io/opendatahub-operator/v2/pkg/client/clientset/versioned"
	internalinterfaces "github.com/opendatahub-io/opendatahub-operator/v2/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/opendatahub-io/opendatahub-operator/v2/pkg/client/listers/dscinitialization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// DSCInitializationInformer provides access to a shared informer and lister for
// DSCInitializations.
type DSCInitializationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.DSCInitializationLister
}

type dSCInitializationInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewDSCInitializationInformer constructs a new informer for DSCInitialization type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewDSCInitializationInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredDSCInitializationInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredDSCInitializationInformer constructs a new informer for DSCInitialization type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredDSCInitializationInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.DscinitializationV1().DSCInitializations().List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.DscinitializationV1().DSCInitializations().Watch(context.TODO(), options)
			},
		},
		&dscinitializationv1.DSCInitialization{},
		resyncPeriod,
		indexers,
	)
}

func (f *dSCInitializationInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredDSCInitializationInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *dSCInitializationInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&dscinitializationv1.DSCInitialization{}, f.defaultInformer)
}

func (f *dSCInitializationInformer) Lister() v1.DSCInitializationLister {
	return v1.NewDSCInitializationLister(f.Informer().GetIndexer())
}
