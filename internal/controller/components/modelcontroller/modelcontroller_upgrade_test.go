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

//nolint:testpackage
package modelcontroller

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"

	. "github.com/onsi/gomega"
)

func TestCleanupLegacyDeployment_DeletesLegacy(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "odh-applications"},
	}

	legacyDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-model-controller",
			Namespace: "odh-applications",
			Labels:    map[string]string{"old-label": "value"},
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(dsci, legacyDeployment))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupLegacyDeployment(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var d appsv1.Deployment
	err = cli.Get(ctx, client.ObjectKey{Name: "odh-model-controller", Namespace: "odh-applications"}, &d)
	g.Expect(k8serr.IsNotFound(err)).To(BeTrue())
}

func TestCleanupLegacyDeployment_SkipsNewDeployment(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "odh-applications"},
	}

	newDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-model-controller",
			Namespace: "odh-applications",
			Labels:    map[string]string{labels.PlatformPartOf: componentApi.ModelControllerComponentName},
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(dsci, newDeployment))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupLegacyDeployment(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var d appsv1.Deployment
	err = cli.Get(ctx, client.ObjectKey{Name: "odh-model-controller", Namespace: "odh-applications"}, &d)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestCleanupLegacyDeployment_HandlesNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "odh-applications"},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(dsci))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupLegacyDeployment(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}
