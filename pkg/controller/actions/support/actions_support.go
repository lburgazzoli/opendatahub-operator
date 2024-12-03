package support

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	"reflect"
	"runtime"
	"strings"
)

const (
	packageActionNamePrefix      = "/pkg/controller/actions/"
	packageActionNameSuffix      = ".(*Action).run-fm"
	packageControllersNamePrefix = "/controllers/components/"
)

func NameOf(fn actions.Fn) string {
	// The function name includes the full package, i.e:
	//
	//   github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/kustomize.(*Action).run-fm
	//   github.com/opendatahub-io/opendatahub-operator/v2/controllers/components/modelregistry.updateStatus
	//
	// Which gets normalized to i.e:
	//
	//   render_kustomize
	//   modelregistry_updateStatus
	//
	_ = reflect.TypeOf(fn).PkgPath()
	n := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

	if i := strings.Index(n, packageControllersNamePrefix); i != -1 {
		n = n[i+len(packageControllersNamePrefix):]

		if i := strings.Index(n, "/"); i != -1 {
			n = n[i+1:]
		}
		if i := strings.Index(n, "."); i != -1 {
			n = n[i+1:]
		}

		n = strings.ReplaceAll(n, ".", "_")
	} else if i := strings.Index(n, packageActionNamePrefix); i != -1 {
		n = n[i+len(packageActionNamePrefix):]

		if i := strings.Index(n, packageActionNameSuffix); i != -1 {
			n = n[:i]
		}

		n = strings.ReplaceAll(n, "/", "_")
	}

	return n
}
