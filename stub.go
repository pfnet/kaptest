package kaptest

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// resources name from API Endpoint. ex: "deployments", "services"
func stubResource() string {
	return ""
}

func stubSubResource() string {
	return ""
}

func stubOperationOptions() runtime.Object {
	return nil
}

func stubIsDryRun() bool {
	return false
}

type authz struct{}

func (*authz) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	return authorizer.DecisionAllow, "reason: stub", nil
}

func stubAuthz() authorizer.Authorizer {
	return &authz{}
}
