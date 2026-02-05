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
package datasciencecluster

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"

	. "github.com/onsi/gomega"
)

func TestCleanupDeprecatedRoleBindings_DeletesMatching(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "odh-applications"},
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-applications",
			Namespace: "odh-applications",
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(dsci, rb))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedRoleBindings(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var deletedRB rbacv1.RoleBinding
	err = cli.Get(ctx, client.ObjectKey{Name: "odh-applications", Namespace: "odh-applications"}, &deletedRB)
	g.Expect(k8serr.IsNotFound(err)).To(BeTrue())
}

func TestCleanupDeprecatedRoleBindings_HandlesNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "odh-applications"},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(dsci))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedRoleBindings(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestCleanupDeprecatedRoleBindings_NoDSCI(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedRoleBindings(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestMigrateHardwareProfiles_NoDSCI(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateHardwareProfiles(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestMigrateHardwareProfiles_EmptyAppNamespace(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: ""},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(dsci))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateHardwareProfiles(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}
