package kaptest

import (
	"testing"

	v1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/authentication/user"
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

var simpleDeployment = &appsv1.Deployment{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "simpleDeployment",
		Namespace: "default",
	},
	Spec: appsv1.DeploymentSpec{
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

func deploymentWithReplicas(deployment *appsv1.Deployment, replicas int) *appsv1.Deployment {
	replicasInt32 := int32(replicas)
	dep := deployment.DeepCopy()
	dep.Spec.Replicas = &replicasInt32
	return dep
}

func TestCompilePolicyNotFail(t *testing.T) {
	compilePolicy(simplePolicy)
}

func TestSimplePolicy(t *testing.T) {
	validator := NewValidator(simplePolicy)
	cases := []struct {
		name           string
		object         runtime.Object
		expectedResult validating.PolicyDecisionEvaluation
	}{
		{"deployment with replica 5", deploymentWithReplicas(simpleDeployment, 5), validating.EvalAdmit},
		{"deployment with replica 6", deploymentWithReplicas(simpleDeployment, 6), validating.EvalDeny},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ValidationParams{Object: tt.object})
			if err != nil {
				t.Errorf("validate finished with error: %v", err)
			}
			if len(result.Decisions) != 1 {
				t.Errorf("decision length is expected to be 1")
			}
			decision := result.Decisions[0]
			if tt.expectedResult != decision.Evaluation {
				t.Errorf("decision evaluation is expected to be %s, but got %s", tt.expectedResult, decision.Evaluation)
			}
			if decision.Action == validating.ActionDeny && decision.Message != simplePolicyMessage {
				t.Errorf("decision message is expected to be %s, but got %s", simplePolicyMessage, decision.Message)
			}
		})
	}
}

func TestPolicyWithVariable(t *testing.T) {
	simpleValidator := NewValidator(simplePolicy)
	policyWithVar := simplePolicy.DeepCopy()
	policyWithVar.Spec.Validations = []v1.Validation{
		{Expression: "variables.replicas <= 5", Message: simplePolicyMessage},
	}
	policyWithVar.Spec.Variables = []v1.Variable{
		{Name: "replicas", Expression: "has(object.spec.replicas) ? object.spec.replicas : 1"},
	}
	validatorWithVar := NewValidator(*policyWithVar)
	cases := []struct {
		name            string
		validator       *validator
		object          runtime.Object
		expectedResult  validating.PolicyDecisionEvaluation
		expectedMessage string
	}{
		{"replica: null with simple validator", simpleValidator, simpleDeployment, validating.EvalError, "expression 'object.spec.replicas <= 5' resulted in error: no such key: replicas"},
		{"replica: null with validatorWithVar", validatorWithVar, simpleDeployment, validating.EvalAdmit, ""},
		{"replica: 5 with validatorWithVar", validatorWithVar, deploymentWithReplicas(simpleDeployment, 5), validating.EvalAdmit, ""},
		{"replica: 6 with validatorWithVar", validatorWithVar, deploymentWithReplicas(simpleDeployment, 6), validating.EvalDeny, simplePolicyMessage},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.validator.Validate(ValidationParams{Object: tt.object})
			if err != nil {
				t.Errorf("validate finished with error: %v", err)
			}
			if len(result.Decisions) != 1 {
				t.Errorf("decision length is expected to be 1")
			}
			decision := result.Decisions[0]
			if tt.expectedResult != decision.Evaluation {
				t.Errorf("decision evaluation is expected to be %s, but got %s", tt.expectedResult, decision.Evaluation)
			}
			if decision.Action == validating.ActionDeny && decision.Message != tt.expectedMessage {
				t.Errorf("decision message is expected to be %s, but got %s", tt.expectedMessage, decision.Message)
			}
		})
	}
}

func TestMatchCondition(t *testing.T) {
	policyWithMatchCondition := simplePolicy.DeepCopy()
	policyWithMatchCondition.Spec.MatchConditions = []v1.MatchCondition{
		{Name: "app label matches", Expression: "object.metadata.labels.app.startsWith('match')"},
		{Name: "app label is same as matchLabels.app", Expression: "object.spec.selector.matchLabels.app == object.metadata.labels.app"},
	}

	appLabelNotMatch := simpleDeployment.DeepCopy()
	appLabelNotMatch.ObjectMeta.Labels = map[string]string{
		"app": "notMatchApp",
	}
	appLabelNotMatch.Spec.Selector.MatchLabels = map[string]string{
		"app": "notMatchApp",
	}

	matchLabelsNotMatch := simpleDeployment.DeepCopy()
	matchLabelsNotMatch.ObjectMeta.Labels = map[string]string{
		"app": "matchApp",
	}
	matchLabelsNotMatch.Spec.Selector.MatchLabels = map[string]string{
		"app": "different from metadata.labels.app",
	}

	deploymentMatch := simpleDeployment.DeepCopy()
	deploymentMatch.ObjectMeta.Labels = map[string]string{
		"app": "matchApp",
	}
	deploymentMatch.Spec.Selector.MatchLabels = map[string]string{
		"app": "matchApp",
	}

	badDeployment := deploymentWithReplicas(deploymentMatch, 6)
	goodDeployment := deploymentWithReplicas(deploymentMatch, 5)

	validator := NewValidator(*policyWithMatchCondition)

	cases := []struct {
		name                string
		object              runtime.Object
		matches             bool
		failedConditionName string
		expectedResult      validating.PolicyDecisionEvaluation
	}{
		{"app label not match", appLabelNotMatch, false, "app label matches", validating.EvalAdmit},
		{"matchLabels.app not match", matchLabelsNotMatch, false, "app label is same as matchLabels.app", validating.EvalAdmit},
		{"deployment with replica 5 match", goodDeployment, true, "", validating.EvalAdmit},
		{"deployment with replica 6 match", badDeployment, true, "", validating.EvalDeny},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			matchResult, err := validator.EvalMatchCondition(ValidationParams{Object: tt.object})
			if err != nil {
				t.Errorf("eval match condition failed with error: %v", err)
			}
			if tt.matches != matchResult.Matches {
				t.Errorf("match result is expected to be %t, but got %t", tt.matches, matchResult.Matches)
			}
			if !tt.matches && tt.failedConditionName != matchResult.FailedConditionName {
				t.Errorf("match failed condition name is expected to be %s, but got %s", tt.failedConditionName, matchResult.FailedConditionName)
			}
			result, err := validator.Validate(ValidationParams{Object: tt.object})
			if err != nil {
				t.Errorf("validate finished with error: %v", err)
			}
			if !tt.matches {
				if len(result.Decisions) != 0 {
					t.Errorf("object is NOT expected to match but got decisions: %v", result.Decisions)
				}
				return
			}
			if len(result.Decisions) != 1 {
				t.Errorf("decision length is expected to be 1")
			}
			decision := result.Decisions[0]
			if tt.expectedResult != decision.Evaluation {
				t.Errorf("decision evaluation is expected to be %s, but got %s", tt.expectedResult, decision.Evaluation)
			}
			if decision.Action == validating.ActionDeny && decision.Message != simplePolicyMessage {
				t.Errorf("decision message is expected to be %s, but got %s", simplePolicyMessage, decision.Message)
			}
		})
	}
}

func TestPolicyWithParam(t *testing.T) {
	conf := &corev1.ConfigMap{
		Data: map[string]string{
			"maxReplicas": "8",
		},
	}
	messageExpression := "'object.spec.replicas should less or equal to ' + params.data.maxReplicas"
	expectedMessage := "object.spec.replicas should less or equal to 8"
	policyWithParam := simplePolicy.DeepCopy()
	policyWithParam.Spec.Validations = []v1.Validation{
		{Expression: "object.spec.replicas <= int(params.data.maxReplicas)", MessageExpression: messageExpression},
	}
	policyWithParam.Spec.ParamKind = &v1.ParamKind{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}
	validator := NewValidator(*policyWithParam)

	cases := []struct {
		name           string
		object         runtime.Object
		expectedResult validating.PolicyDecisionEvaluation
	}{
		{"deployment with replica 8", deploymentWithReplicas(simpleDeployment, 8), validating.EvalAdmit},
		{"deployment with replica 9", deploymentWithReplicas(simpleDeployment, 9), validating.EvalDeny},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ValidationParams{Object: tt.object, ParamObj: conf})
			if err != nil {
				t.Errorf("validate finished with error: %v", err)
			}
			if len(result.Decisions) != 1 {
				t.Errorf("decision length is expected to be 1")
			}
			decision := result.Decisions[0]
			if tt.expectedResult != decision.Evaluation {
				t.Errorf("decision evaluation is expected to be %s, but got %s", tt.expectedResult, decision.Evaluation)
			}
			if decision.Action == validating.ActionDeny && decision.Message != expectedMessage {
				t.Errorf("decision message is expected to be %s, but got %s", expectedMessage, decision.Message)
			}
		})
	}
}

func TestPolicyWithUserInfo(t *testing.T) {
	policyWithUserInfo := simplePolicy.DeepCopy()
	message := "user must be a member of admin"
	policyWithUserInfo.Spec.Validations = []v1.Validation{
		{Expression: "'admin' in request.userInfo.groups", Message: message},
	}
	validator := NewValidator(*policyWithUserInfo)
	cases := []struct {
		name           string
		group          string
		expectedResult validating.PolicyDecisionEvaluation
	}{
		{"group is member, not admin", "member", validating.EvalDeny},
		{"group is admin", "admin", validating.EvalAdmit},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ValidationParams{
				Object: simpleDeployment, UserInfo: &user.DefaultInfo{Groups: []string{tt.group}},
			})
			if err != nil {
				t.Errorf("validate finished with error: %v", err)
			}
			if len(result.Decisions) != 1 {
				t.Errorf("decision length is expected to be 1")
			}
			decision := result.Decisions[0]
			if tt.expectedResult != decision.Evaluation {
				t.Errorf("decision evaluation is expected to be %s, but got %s", tt.expectedResult, decision.Evaluation)
			}
			if decision.Action == validating.ActionDeny && decision.Message != message {
				t.Errorf("decision message is expected to be %s, but got %s", message, decision.Message)
			}
		})
	}
}

func TestDeletionCase(t *testing.T) {
	policyAboutDeletion := simplePolicy.DeepCopy()
	policyAboutDeletion.Spec.Validations = []v1.Validation{
		{Expression: "oldObject.spec.replicas <= 5", Message: simplePolicyMessage},
	}
	validator := NewValidator(*policyAboutDeletion)
	cases := []struct {
		name           string
		oldObject      runtime.Object
		expectedResult validating.PolicyDecisionEvaluation
	}{
		{"deployment with replica 5", deploymentWithReplicas(simpleDeployment, 5), validating.EvalAdmit},
		{"deployment with replica 6", deploymentWithReplicas(simpleDeployment, 6), validating.EvalDeny},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(ValidationParams{OldObject: tt.oldObject})
			if err != nil {
				t.Errorf("validate finished with error: %v", err)
			}
			if len(result.Decisions) != 1 {
				t.Errorf("decision length is expected to be 1")
			}
			decision := result.Decisions[0]
			if tt.expectedResult != decision.Evaluation {
				t.Errorf("decision evaluation is expected to be %s, but got %s", tt.expectedResult, decision.Evaluation)
			}
			if decision.Action == validating.ActionDeny && decision.Message != simplePolicyMessage {
				t.Errorf("decision message is expected to be %s, but got %s", simplePolicyMessage, decision.Message)
			}
		})
	}
}
