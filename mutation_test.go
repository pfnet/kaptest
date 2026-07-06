/*
Copyright 2025 Preferred Networks, Inc.

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

package kaptest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/ptr"
)

func simpleMutatingPolicyAndBinding() (*v1beta1.MutatingAdmissionPolicy, *v1beta1.MutatingAdmissionPolicyBinding) {
	mut := &v1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simplePolicy",
		},
		Spec: v1beta1.MutatingAdmissionPolicySpec{
			FailurePolicy:      ptr.To(v1beta1.Fail),
			ReinvocationPolicy: v1beta1.IfNeededReinvocationPolicy,
			MatchConstraints: &v1beta1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				MatchPolicy:       ptr.To(v1beta1.Equivalent),
				ResourceRules: []v1beta1.NamedRuleWithOperations{
					{
						RuleWithOperations: v1.RuleWithOperations{
							Rule: v1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
							Operations: []v1.OperationType{"*"},
						},
					},
				},
			},
			Mutations: []v1beta1.Mutation{
				{
					PatchType: v1beta1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &v1beta1.ApplyConfiguration{
						Expression: `
							Object{
								metadata: Object.metadata{
									labels: {"environment": "test"}
								}
							}
						`,
					},
				},
			},
		},
	}
	mut.GetObjectKind().SetGroupVersionKind(v1beta1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy"))

	binding := &v1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simplePolicyBinding",
		},
		Spec: v1beta1.MutatingAdmissionPolicyBindingSpec{
			PolicyName: mut.Name,
			MatchResources: &v1beta1.MatchResources{
				MatchPolicy:       ptr.To(v1beta1.Equivalent),
				ObjectSelector:    &metav1.LabelSelector{},
				NamespaceSelector: &metav1.LabelSelector{},
			},
		},
	}
	binding.GetObjectKind().SetGroupVersionKind(v1beta1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding"))

	return mut, binding
}

func simpleMutatingDeploymentParam() MutationParams {
	return MutationParams{
		Object: &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "d1",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{},
		},
		OldObject: nil,
		ParamObj:  nil,
		NamespaceObj: &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
			Spec: corev1.NamespaceSpec{},
		},
		UserInfo: nil,
	}
}

func TestCompileMutatingPolicy_NotFail(t *testing.T) {
	policy, _ := simpleMutatingPolicyAndBinding()
	evaluator := compileMutatitionAddmissionPolicy(policy)
	if evaluator.Error != nil {
		t.Errorf("failed to compile policy: %q", evaluator.Error)
	}
}

func TestMutator_Mutate_SimplePolicy(t *testing.T) {
	p := simpleMutatingDeploymentParam()
	expectedObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d1",
			Namespace: "default",
			Labels:    map[string]string{"environment": "test"},
		},
		Spec: appsv1.DeploymentSpec{},
	}

	policy, _ := simpleMutatingPolicyAndBinding()
	mutator, err := NewMutator(policy)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := mutator.Mutate(p)
	if err != nil {
		t.Fatal(err)
	}

	matchedParam, err := mutator.EvalMatchCondition(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(matchedParam) != 1 {
		t.Errorf("expected %d matches, but %d matches", 1, len(matchedParam))
	}
	if matchedParam[0].Invocation.Param != nil {
		t.Errorf("unexpected param is matched")
	}

	if !equality.Semantic.DeepEqual(obj, expectedObj) {
		t.Errorf("unexpected result, got diff:\n%s\n", cmp.Diff(expectedObj, obj))
	}
}

func TestMutator_Mutate_SimplePolicy_WithVariable(t *testing.T) {
	p := simpleMutatingDeploymentParam()
	expectedObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d1",
			Namespace: "default",
			Labels:    map[string]string{"environment": "d1-mutated"},
		},
		Spec: appsv1.DeploymentSpec{},
	}

	policy, _ := simpleMutatingPolicyAndBinding()
	policy.Spec.Variables = []v1beta1.Variable{
		{Name: "envValue", Expression: "has(object.metadata.name) ? object.metadata.name+\"-mutated\" : \"unmutated\""},
	}
	policy.Spec.Mutations = []v1beta1.Mutation{
		{
			PatchType: v1beta1.PatchTypeApplyConfiguration,
			ApplyConfiguration: &v1beta1.ApplyConfiguration{
				Expression: `
				Object{
					metadata: Object.metadata{
						labels: {"environment": variables.envValue}
					}
				}`,
			},
		},
	}
	mutator, err := NewMutator(policy)
	if err != nil {
		t.Fatal(err)
	}

	matchedParam, err := mutator.EvalMatchCondition(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(matchedParam) != 1 {
		t.Errorf("expected %d matches, but %d matches", 1, len(matchedParam))
	}
	if matchedParam[0].Invocation.Param != nil {
		t.Errorf("unexpected param is matched")
	}

	obj, err := mutator.Mutate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !equality.Semantic.DeepEqual(obj, expectedObj) {
		t.Errorf("unexpected result, got diff:\n%s\n", cmp.Diff(expectedObj, obj))
	}
}

func TestMutator_Mutate_SimplePolicy_WithParam(t *testing.T) {
	p := simpleMutatingDeploymentParam()
	expectedObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d1",
			Namespace: "default",
			Labels:    map[string]string{"environment": "env-from-configmap"},
		},
		Spec: appsv1.DeploymentSpec{},
	}

	p.ParamObj = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mutatedValues",
			Namespace: "default",
		},
		Data: map[string]string{
			"envValue": "env-from-configmap",
		},
	}

	policy, _ := simpleMutatingPolicyAndBinding()
	policy.Spec.ParamKind = &v1beta1.ParamKind{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}
	policy.Spec.Mutations = []v1beta1.Mutation{
		{
			PatchType: v1beta1.PatchTypeApplyConfiguration,
			ApplyConfiguration: &v1beta1.ApplyConfiguration{
				Expression: `
				Object{
					metadata: Object.metadata{
						labels: {"environment": params.data.envValue}
					}
				}`,
			},
		},
	}
	mutator, err := NewMutator(policy)
	if err != nil {
		t.Fatal(err)
	}

	matchedHooks, err := mutator.EvalMatchCondition(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(matchedHooks) != 1 {
		t.Errorf("expected %d matches, but %d matches", 1, len(matchedHooks))
	}
	actualParamObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(matchedHooks[0].Invocation.Param)
	if err != nil {
		t.Fatal(err)
	}
	expectedParamObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p.ParamObj)
	if err != nil {
		t.Fatal(err)
	}
	if !equality.Semantic.DeepEqual(actualParamObj, expectedParamObj) {
		t.Errorf("unexpected param is matched")
	}

	obj, err := mutator.Mutate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !equality.Semantic.DeepEqual(obj, expectedObj) {
		t.Errorf("unexpected result, got diff:\n%s\n", cmp.Diff(expectedObj, obj))
	}
}

func TestMutator_Mutate_SimplePolicy_WithUserInfo(t *testing.T) {
	p := simpleMutatingDeploymentParam()
	expectedObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "d1",
			Namespace: "default",
			Labels:    map[string]string{"username": "test-user"},
		},
		Spec: appsv1.DeploymentSpec{},
	}
	p.UserInfo = &user.DefaultInfo{
		Name: "test-user",
	}

	policy, _ := simpleMutatingPolicyAndBinding()
	policy.Spec.Mutations = []v1beta1.Mutation{
		{
			PatchType: v1beta1.PatchTypeApplyConfiguration,
			ApplyConfiguration: &v1beta1.ApplyConfiguration{
				Expression: `
				Object{
					metadata: Object.metadata{
						labels: {"username":  request.userInfo.username}
					}
				}`,
			},
		},
	}
	mutator, err := NewMutator(policy)
	if err != nil {
		t.Fatal(err)
	}

	matchedHooks, err := mutator.EvalMatchCondition(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(matchedHooks) != 1 {
		t.Errorf("expected %d matches, but %d matches", 1, len(matchedHooks))
	}

	obj, err := mutator.Mutate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !equality.Semantic.DeepEqual(obj, expectedObj) {
		t.Errorf("unexpected result, got diff:\n%s\n", cmp.Diff(expectedObj, obj))
	}
}
