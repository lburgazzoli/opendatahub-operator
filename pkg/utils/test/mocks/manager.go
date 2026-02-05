/*
Copyright 2025.

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

//nolint:forcetypeassert,errcheck
package mocks

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Manager is a mock implementation of ctrl.Manager for testing.
type Manager struct {
	mock.Mock
}

func NewMockManager(f func(m *Manager)) *Manager {
	m := new(Manager)
	f(m)
	return m
}

func (m *Manager) GetClient() client.Client {
	return m.Called().Get(0).(client.Client)
}

func (m *Manager) GetScheme() *runtime.Scheme {
	return m.Called().Get(0).(*runtime.Scheme)
}

func (m *Manager) GetRESTMapper() meta.RESTMapper {
	return m.Called().Get(0).(meta.RESTMapper)
}

func (m *Manager) GetConfig() *rest.Config {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*rest.Config)
}

func (m *Manager) GetFieldIndexer() client.FieldIndexer {
	return m.Called().Get(0).(client.FieldIndexer)
}

func (m *Manager) GetEventRecorderFor(_ string) record.EventRecorder {
	return m.Called().Get(0).(record.EventRecorder)
}

func (m *Manager) GetCache() cache.Cache {
	return m.Called().Get(0).(cache.Cache)
}

func (m *Manager) GetLogger() logr.Logger {
	return ctrl.Log
}

func (m *Manager) Add(_ manager.Runnable) error {
	return m.Called().Error(0)
}

func (m *Manager) Elected() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (m *Manager) Start(_ context.Context) error {
	return m.Called().Error(0)
}

func (m *Manager) AddHealthzCheck(_ string, _ healthz.Checker) error {
	return m.Called().Error(0)
}

func (m *Manager) AddMetricsServerExtraHandler(_ string, _ http.Handler) error {
	return m.Called().Error(0)
}

func (m *Manager) AddReadyzCheck(_ string, _ healthz.Checker) error {
	return m.Called().Error(0)
}

func (m *Manager) GetAPIReader() client.Reader {
	return m.Called().Get(0).(client.Reader)
}

func (m *Manager) GetControllerOptions() config.Controller {
	return m.Called().Get(0).(config.Controller)
}

func (m *Manager) GetHTTPClient() *http.Client {
	return m.Called().Get(0).(*http.Client)
}

func (m *Manager) GetWebhookServer() webhook.Server {
	return m.Called().Get(0).(webhook.Server)
}
