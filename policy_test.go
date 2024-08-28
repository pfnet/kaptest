package validating

import (
	"testing"

	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


var failurePolicyFail = v1.Fail
var simplePolicyMessage = "object.spec.replicas should less or equal to 5"
var simplePolicy = v1.ValidatingAdmissionPolicy{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ValidatingAdmissionPolicy",
		APIVersion: "admissionregistration.k8s.io/v1beta1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name:      "simplePolicy",
		Namespace: "default",
	},
	Spec: v1.ValidatingAdmissionPolicySpec{
		FailurePolicy: &failurePolicyFail,
		MatchConstraints: &v1.MatchResources{
			ResourceRules: []v1.NamedRuleWithOperations{
				{
					RuleWithOperations: v1.RuleWithOperations{
						Rule: v1.Rule{
							APIGroups:   []string{"apps"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
						Operations: []v1.OperationType{"CREATE", "UPDATE"},
					},
				},
			},
		},
		Validations: []v1.Validation{
			{Expression: "object.spec.replicas <= 5", Message: simplePolicyMessage},
		},
	},
}

func TestCompilePolicyNotFail(t *testing.T) {
	compilePolicy(simplePolicy)
}
