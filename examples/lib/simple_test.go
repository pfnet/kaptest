package lib

import (
	"kaptest"
	"testing"

	v1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

var failurePolicyFail = v1.Fail
var simplePolicyMessage = "object.spec.replicas should less or equal to 5"
var simplePolicy = v1.ValidatingAdmissionPolicy{
	Spec: v1.ValidatingAdmissionPolicySpec{
		FailurePolicy: &failurePolicyFail,
		Validations: []v1.Validation{
			{Expression: "object.spec.replicas <= 5", Message: simplePolicyMessage},
		},
	},
}

var replicas6 = int32(6)
var simpleDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "simpleDeployment",
		Namespace: "default",
	},
	Spec: appsv1.DeploymentSpec{
		Replicas: &replicas6,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "nginx"},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "nginx"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.14.2",
						Ports: []corev1.ContainerPort{
							{ContainerPort: 80},
						},
					},
				},
			},
		},
	},
}

func TestSimplePolicy(t *testing.T) {
	validator := kaptest.NewValidator(&simplePolicy)
	result, _ := validator.Validate(kaptest.ValidationParams{Object: simpleDeployment})
	decision := result.Decisions[0]
	expectedResult := validating.EvalDeny
	if expectedResult != decision.Evaluation {
		t.Errorf("decision evaluation is expected to be %s, but got %s", expectedResult, decision.Evaluation)
	}
}
