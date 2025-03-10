package types

import "k8s.io/apimachinery/pkg/runtime/schema"

type gvkInfo struct {
	owned   bool
	partial bool
}

func NewBaseController() *BaseController {
	return &BaseController{
		gvks: make(map[schema.GroupVersionKind]gvkInfo),
	}
}

type BaseController struct {
	gvks map[schema.GroupVersionKind]gvkInfo
}

func (m *BaseController) update(gvk schema.GroupVersionKind, fn func(gvki *gvkInfo)) {
	gvki := m.gvks[gvk]

	fn(&gvki)

	m.gvks[gvk] = gvki
}

func (m *BaseController) SetOwnedType(gvk schema.GroupVersionKind, owned bool) {
	if m == nil {
		return
	}

	m.update(gvk, func(gvki *gvkInfo) {
		gvki.owned = owned
	})
}

func (m *BaseController) Owns(gvk schema.GroupVersionKind) bool {
	if m == nil {
		return false
	}

	if i, ok := m.gvks[gvk]; ok {
		return i.owned
	}

	return false
}

func (m *BaseController) SetPartialType(gvk schema.GroupVersionKind, partial bool) {
	if m == nil {
		return
	}

	m.update(gvk, func(gvki *gvkInfo) {
		gvki.partial = partial
	})
}

func (m *BaseController) IsPartial(gvk schema.GroupVersionKind) bool {
	if m == nil {
		return false
	}

	if i, ok := m.gvks[gvk]; ok {
		return i.partial
	}

	return false
}
