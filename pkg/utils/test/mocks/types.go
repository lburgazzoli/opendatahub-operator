//nolint:errcheck,forcetypeassert
package mocks

import (
	"context"
	"fmt"
	"strings"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/stretchr/testify/mock"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	rrtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/operatorconfig"
)

// MockComponentHandler is a testify/mock implementation of cr.ComponentHandler.
type MockComponentHandler struct {
	mock.Mock
}

func (m *MockComponentHandler) Init(platform common.Platform, cfg operatorconfig.OperatorSettings) error {
	return m.Called(platform, cfg).Error(0)
}

func (m *MockComponentHandler) GetName() string {
	return m.Called().String(0)
}

func (m *MockComponentHandler) GroupVersionKind() schema.GroupVersionKind {
	return m.Called().Get(0).(schema.GroupVersionKind)
}

func (m *MockComponentHandler) NewCRObject(ctx context.Context, cli client.Client, dsc *dscv2.DataScienceCluster) (common.PlatformObject, error) {
	args := m.Called(ctx, cli, dsc)
	if args.Get(1) != nil {
		return nil, args.Get(1).(error)
	}
	if args.Get(0) == nil {
		return nil, nil
	}
	return args.Get(0).(common.PlatformObject), nil
}

func (m *MockComponentHandler) NewComponentReconciler(ctx context.Context, mgr ctrl.Manager, tracker *provision.RunlevelTracker) error {
	return m.Called(ctx, mgr, tracker).Error(0)
}

func (m *MockComponentHandler) UpdateDSCStatus(ctx context.Context, rr *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
	args := m.Called(ctx, rr)
	if args.Get(1) != nil {
		return "", args.Get(1).(error)
	}
	return args.Get(0).(metav1.ConditionStatus), nil
}

func (m *MockComponentHandler) IsEnabled(dsc *dscv2.DataScienceCluster) bool {
	return m.Called(dsc).Bool(0)
}

// NewMockComponentHandler creates a MockComponentHandler and passes it to f
// for configuration. Only the expectations set in f are registered.
func NewMockComponentHandler(f func(*MockComponentHandler)) *MockComponentHandler {
	m := new(MockComponentHandler)
	f(m)
	return m
}

// NewDefaultMockComponentHandler creates a MockComponentHandler with sensible defaults.
// name and gvk are used for the most common method expectations.
func NewDefaultMockComponentHandler(name string, gvk schema.GroupVersionKind) *MockComponentHandler {
	m := new(MockComponentHandler)
	m.On("GetName").Return(name).Maybe()
	m.On("GroupVersionKind").Return(gvk).Maybe()
	m.On("Init", mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("IsEnabled", mock.Anything).Return(true).Maybe()
	m.On("NewCRObject", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	m.On("NewComponentReconciler", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("UpdateDSCStatus", mock.Anything, mock.Anything).Return(metav1.ConditionTrue, nil).Maybe()
	return m
}

// MockModuleHandler is a testify/mock implementation of modules.ModuleHandler.
type MockModuleHandler struct {
	mock.Mock
}

func (m *MockModuleHandler) GetName() string {
	return m.Called().String(0)
}

func (m *MockModuleHandler) IsEnabled(platform *modules.PlatformContext) bool {
	return m.Called(platform).Bool(0)
}

func (m *MockModuleHandler) GetGroupVersionKind() schema.GroupVersionKind {
	return m.Called().Get(0).(schema.GroupVersionKind)
}

func (m *MockModuleHandler) GetOperatorManifests(platform *modules.PlatformContext) modules.OperatorManifests {
	return m.Called(platform).Get(0).(modules.OperatorManifests)
}

func (m *MockModuleHandler) BuildModuleCR(ctx context.Context, cli client.Client, platform *modules.PlatformContext) (*unstructured.Unstructured, error) {
	args := m.Called(ctx, cli, platform)
	if args.Get(1) != nil {
		return nil, args.Get(1).(error)
	}
	if args.Get(0) == nil {
		return nil, nil
	}
	return args.Get(0).(*unstructured.Unstructured), nil
}

func (m *MockModuleHandler) GetRelatedImages() []string {
	return m.Called().Get(0).([]string)
}

func (m *MockModuleHandler) GetModuleStatus(ctx context.Context, cli client.Client) (*modules.ModuleStatus, error) {
	args := m.Called(ctx, cli)
	if args.Get(1) != nil {
		return nil, args.Get(1).(error)
	}
	if args.Get(0) == nil {
		return nil, nil
	}
	return args.Get(0).(*modules.ModuleStatus), nil
}

func (m *MockModuleHandler) GetModuleCRState(ctx context.Context, cli client.Client) (modules.CRState, error) {
	args := m.Called(ctx, cli)
	if args.Get(1) != nil {
		return 0, args.Get(1).(error)
	}
	return args.Get(0).(modules.CRState), nil
}

func (m *MockModuleHandler) DeleteModuleCR(ctx context.Context, cli client.Client) error {
	return m.Called(ctx, cli).Error(0)
}

func (m *MockModuleHandler) DeleteOperatorResources(ctx context.Context, cli client.Client, platform *modules.PlatformContext) error {
	return m.Called(ctx, cli, platform).Error(0)
}

func (m *MockModuleHandler) ApplyManagementState(ctx *modules.PlatformContext, spec *configv1alpha1.PlatformModules) {
	m.Called(ctx, spec)
}

// NewMockModuleHandler creates a MockModuleHandler and passes it to f
// for configuration. Only the expectations set in f are registered.
func NewMockModuleHandler(f func(*MockModuleHandler)) *MockModuleHandler {
	m := new(MockModuleHandler)
	f(m)
	return m
}

// NewDefaultMockModuleHandler creates a MockModuleHandler with sensible defaults.
// name and gvk are used for the most common method expectations.
func NewDefaultMockModuleHandler(name string, gvk schema.GroupVersionKind) *MockModuleHandler {
	m := new(MockModuleHandler)
	m.On("GetName").Return(name).Maybe()
	m.On("GetGroupVersionKind").Return(gvk).Maybe()
	m.On("IsEnabled", mock.Anything).Return(true).Maybe()
	m.On("GetOperatorManifests", mock.Anything).Return(modules.OperatorManifests{}).Maybe()
	m.On("BuildModuleCR", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	m.On("GetRelatedImages").Return([]string{}).Maybe()
	m.On("GetModuleStatus", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	m.On("GetModuleCRState", mock.Anything, mock.Anything).Return(modules.CRStateAbsent, nil).Maybe()
	m.On("DeleteModuleCR", mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("DeleteOperatorResources", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	m.On("ApplyManagementState", mock.Anything, mock.Anything).Maybe()
	return m
}

type MockController struct {
	mock.Mock
}

func (m *MockController) Owns(gvk schema.GroupVersionKind) bool {
	return m.Called(gvk).Bool(0)
}

func (m *MockController) GetClient() client.Client {
	return m.Called().Get(0).(client.Client)
}

func (m *MockController) GetDiscoveryClient() discovery.DiscoveryInterface {
	return m.Called().Get(0).(discovery.DiscoveryInterface)
}

func (m *MockController) GetDynamicClient() dynamic.Interface {
	return m.Called().Get(0).(dynamic.Interface)
}

func (m *MockController) IsDynamicOwnershipEnabled() bool {
	args := m.Called()
	if len(args) == 0 {
		return false
	}
	return args.Bool(0)
}

func (m *MockController) IsExcludedFromDynamicOwnership(gvk schema.GroupVersionKind) bool {
	args := m.Called(gvk)
	if len(args) == 0 {
		return false
	}
	return args.Bool(0)
}

func (m *MockController) AddDynamicOwnedType(gvk schema.GroupVersionKind) {
	m.Called(gvk)
}

func NewMockController(f func(m *MockController)) *MockController {
	m := new(MockController)
	f(m)

	m.On("Owns", mock.Anything).Return(false).Maybe()
	m.On("IsExcludedFromDynamicOwnership", mock.Anything).Return(false).Maybe()
	m.On("IsDynamicOwnershipEnabled").Return(false).Maybe()
	m.On("AddDynamicOwnedType", mock.Anything).Return().Maybe()

	return m
}

// NewMockCRD creates a mock CRD with the specified parameters.
func NewMockCRD(group, version, kind, componentName string) *apiextv1.CustomResourceDefinition {
	return &apiextv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(fmt.Sprintf("%ss.%s", kind, group)),
			Labels: map[string]string{
				labels.ODH.Component(componentName): labels.True,
			},
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:   kind,
				Plural: strings.ToLower(kind) + "s",
			},
			Scope: apiextv1.ClusterScoped,
			Versions: []apiextv1.CustomResourceDefinitionVersion{
				{
					Name:    version,
					Served:  true,
					Storage: true,
					Schema: &apiextv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}
}

// Ensure interfaces are satisfied at compile time.
var (
	_ operatorv1.ManagementState = "" // keep import
)
