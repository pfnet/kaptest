/*
Copyright 2024 Preferred Networks, Inc.

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

package kaptest_test

import (
	"fmt"

	"github.com/pfnet/kaptest"
	v1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func ExampleValidator_EvalMatchCondition() {
	samplePolicy := v1.ValidatingAdmissionPolicy{
		Spec: v1.ValidatingAdmissionPolicySpec{
			MatchConditions: []v1.MatchCondition{
				{Name: "is-mutable", Expression: "oldObject.?metadata.?labels['immutable'].orValue('') != 'true'"},
			},
		},
	}
	sampleDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "simple",
			Labels: map[string]string{"immutable": "true"},
		},
		// Spec: appsv1.DeploymentSpec{...}
	}

	validator := kaptest.NewValidator(&samplePolicy)
	result := validator.EvalMatchCondition(kaptest.ValidationParams{OldObject: sampleDeployment})
	fmt.Printf("match: %t, condition: %q\n", result.Matches, result.FailedConditionName)
	// Output: match: false, condition: "is-mutable"
}

func ExampleValidator_Validate() {
	samplePolicy := v1.ValidatingAdmissionPolicy{
		Spec: v1.ValidatingAdmissionPolicySpec{
			Validations: []v1.Validation{
				{Expression: "object.spec.replicas < 5", Message: "spec.replicas should be less than 5"},
			},
		},
	}
	sampleDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "simple"},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(6)),
			// LabelSelector, PodTemplateSpec...
		},
	}

	validator := kaptest.NewValidator(&samplePolicy)
	result, _ := validator.Validate(kaptest.ValidationParams{Object: sampleDeployment})
	fmt.Println(result.Decisions[0].Evaluation)
	// Output: deny
}
