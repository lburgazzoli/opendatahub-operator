package actions

import (
	"context"
	"fmt"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

//
// Common
//

const (
	ActionGroup = "action"
)

type Action interface {
	fmt.Stringer
	Execute(ctx context.Context, rr *types.ReconciliationRequest) error
}
