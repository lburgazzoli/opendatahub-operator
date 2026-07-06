package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ PlatformObject = &UnstructuredModule{}
var _ WithReleases = &UnstructuredModule{}

// UnstructuredModule adapts an *unstructured.Unstructured to the PlatformObject
// and WithReleases interfaces for tracked CRs that expose status.releases.
type UnstructuredModule struct {
	*UnstructuredPlatformObject
}

// NewUnstructuredModule wraps an existing Unstructured as a release-aware
// PlatformObject.
func NewUnstructuredModule(u *unstructured.Unstructured) *UnstructuredModule {
	return &UnstructuredModule{
		UnstructuredPlatformObject: NewUnstructuredPlatformObject(u),
	}
}

func (o *UnstructuredModule) GetReleaseStatus() *[]ComponentRelease {
	rawReleases, found, err := unstructured.NestedSlice(o.Object, "status", "releases")
	if err != nil || !found {
		return nil
	}

	releases := make([]ComponentRelease, 0, len(rawReleases))
	for _, raw := range rawReleases {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		name, _, _ := unstructured.NestedString(entry, "name")
		if name == "" {
			continue
		}

		version, _, _ := unstructured.NestedString(entry, "version")
		repoURL, _, _ := unstructured.NestedString(entry, "repoUrl")
		releases = append(releases, ComponentRelease{
			Name:    name,
			Version: version,
			RepoURL: repoURL,
		})
	}

	return &releases
}

func (o *UnstructuredModule) SetReleaseStatus(releases []ComponentRelease) {
	raw := make([]any, 0, len(releases))
	for _, release := range releases {
		entry := map[string]any{
			"name": release.Name,
		}
		if release.Version != "" {
			entry["version"] = release.Version
		}
		if release.RepoURL != "" {
			entry["repoUrl"] = release.RepoURL
		}
		raw = append(raw, entry)
	}

	_ = unstructured.SetNestedSlice(o.Object, raw, "status", "releases")
}

func (o *UnstructuredModule) DeepCopyObject() runtime.Object {
	return &UnstructuredModule{
		UnstructuredPlatformObject: NewUnstructuredPlatformObject(o.Unstructured.DeepCopy()),
	}
}
