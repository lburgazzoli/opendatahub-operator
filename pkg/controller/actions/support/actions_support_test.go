package support_test

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/support"
	"testing"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/deploy"

	. "github.com/onsi/gomega"
)

func TestNameOf(t *testing.T) {
	g := NewWithT(t)

	g.Expect(support.NameOf(deploy.NewAction())).To(Equal("deploy"))
}
