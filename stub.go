package kaptest

import (
	"context"

	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type authz struct{}

func (*authz) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	return authorizer.DecisionAllow, "reason: stub", nil
}

func stubAuthz() authorizer.Authorizer {
	return &authz{}
}
