package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"reflect"


	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type ChecksumFn func(_ context.Context, rr *types.ReconciliationRequest) ([]byte, error)

// Action takes a set of manifest locations and render them as Unstructured resources for
// further processing. The Action can eventually cache the results in memory to avoid doing
// a full manifest rendering when not needed.
type Action struct {
	keOpts []kustomize.EngineOptsFn
	ke     *kustomize.Engine

	cacheAnnotation bool
	checksumFn      ChecksumFn
	checksum        []byte
	cachedResources []unstructured.Unstructured
}

type ActionOpts func(*Action)

func WithEngineFS(value filesys.FileSystem) ActionOpts {
	return func(a *Action) {
		a.keOpts = append(a.keOpts, kustomize.WithEngineFS(value))
	}
}

func WithLabel(name string, value string) ActionOpts {
	return func(a *Action) {
		a.keOpts = append(a.keOpts, kustomize.WithEngineRenderOpts(kustomize.WithLabel(name, value)))
	}
}

func WithLabels(values map[string]string) ActionOpts {
	return func(a *Action) {
		a.keOpts = append(a.keOpts, kustomize.WithEngineRenderOpts(kustomize.WithLabels(values)))
	}
}

func WithAnnotation(name string, value string) ActionOpts {
	return func(a *Action) {
		a.keOpts = append(a.keOpts, kustomize.WithEngineRenderOpts(kustomize.WithAnnotation(name, value)))
	}
}

func WithAnnotations(values map[string]string) ActionOpts {
	return func(a *Action) {
		a.keOpts = append(a.keOpts, kustomize.WithEngineRenderOpts(kustomize.WithAnnotations(values)))
	}
}

func WithManifestsOptions(values ...kustomize.EngineOptsFn) ActionOpts {
	return func(action *Action) {
		action.keOpts = append(action.keOpts, values...)
	}
}

func WithCache(addHashAnnotation bool, value ChecksumFn) ActionOpts {
	return func(action *Action) {
		action.cacheAnnotation = addHashAnnotation
		action.checksumFn = value
	}
}

func (a *Action) Execute(ctx context.Context, rr *types.ReconciliationRequest) error {
	checksum, err := a.checksumFn(ctx, rr)
	if err != nil {
		return fmt.Errorf("unable to calculate checksum of reconciliation object: %w", err)
	}

	var result []unstructured.Unstructured

	if len(checksum) != 0 && bytes.Equal(checksum, a.checksum) && len(a.cachedResources) != 0 {
		result = a.cachedResources
	} else {
		res, err := a.render(rr)
		if err != nil {
			return fmt.Errorf("unable to render reconciliation object: %w", err)
		}

		result = res

		if len(checksum) != 0 {
			a.checksum = checksum
			a.cachedResources = result

			for i := range result {
				// Add a letter at the beginning and use URL safe encoding
				digest := "v" + base64.RawURLEncoding.EncodeToString(checksum)

				resources.SetAnnotation(&a.cachedResources[i], annotations.ComponentHash, digest)
			}
		}
	}

	for i := range result {
		// deep copy object so changes done in the pipelines won't
		// alter them
		rr.Resources = append(rr.Resources, *result[i].DeepCopy())
	}

	return nil
}

func (a *Action) render(rr *types.ReconciliationRequest) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, 0)

	for i := range rr.Manifests {
		renderedResources, err := a.ke.Render(
			rr.Manifests[i].ManifestsPath(),
			kustomize.WithNamespace(rr.DSCI.Spec.ApplicationsNamespace),
		)

		if err != nil {
			return nil, err
		}

		result = append(result, renderedResources...)
	}

	return result, nil
}

func (a *Action) String() string {
	return reflect.TypeOf(a).String()
}

func New(opts ...ActionOpts) *Action {
	action := Action{
		checksumFn: func(_ context.Context, rr *types.ReconciliationRequest) ([]byte, error) {
			return nil, nil
		},
	}

	for _, opt := range opts {
		opt(&action)
	}

	action.ke = kustomize.NewEngine(action.keOpts...)

	return &action
}

func DefaultChecksumFn(_ context.Context, rr *types.ReconciliationRequest) ([]byte, error) {
	hash := sha256.New()

	generation := make([]byte, binary.MaxVarintLen64)
	binary.PutVarint(generation, rr.Instance.GetGeneration())

	if _, err := hash.Write(generation); err != nil {
		return nil, fmt.Errorf("unable to calculate checksum of reconciliation object: %w", err)
	}
	if _, err := hash.Write([]byte(rr.Release.Name)); err != nil {
		return nil, fmt.Errorf("unable to calculate checksum of reconciliation object: %w", err)
	}
	if _, err := hash.Write([]byte(rr.Release.Version.String())); err != nil {
		return nil, fmt.Errorf("unable to calculate checksum of reconciliation object: %w", err)
	}

	return hash.Sum(nil), nil
}
