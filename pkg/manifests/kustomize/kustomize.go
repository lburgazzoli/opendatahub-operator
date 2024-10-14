package kustomize

import (
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const (
	defaultKustomizationFileName = "kustomization.yaml"
	defaultKustomizationFilePath = "default"
)

func NewEngine(opts ...RenderOptsFn) *Engine {
	e := Engine{
		k:  krusty.MakeKustomizer(krusty.MakeDefaultOptions()),
		fs: filesys.MakeFsOnDisk(),
		renderOpts: renderOpts{
			kustomizationFileName:    defaultKustomizationFileName,
			kustomizationFileOverlay: defaultKustomizationFilePath,
		},
	}

	for _, fn := range opts {
		fn(&e.renderOpts)
	}

	return &e
}
