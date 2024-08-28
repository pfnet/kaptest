package validating

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// resources name from API Endpoint. ex: "deployments", "services"
func stubResource() string {
	return ""
}

func stubSubResource() string {
	return ""
}

func stubAdmissionOperation() admission.Operation {
	return admission.Create
}

func stubOperationOptions() runtime.Object {
	return nil
}

func stubIsDryRun() bool {
	return false
}

func stubRuntimeCELCostBudget() int64 {
	return 9223372036854775807
}

type authz struct{}

func (*authz) Authorize(ctx context.Context, a authorizer.Attributes) (authorized authorizer.Decision, reason string, err error) {
	return authorizer.DecisionAllow, "reason: stub", nil
}

func stubAuthz() authorizer.Authorizer {
	return &authz{}
}
