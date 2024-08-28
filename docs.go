/*
Module to test Validating Admission Policy.

Example:

	func TestSimplePolicy(t *testing.T) {
		validator := validating.NewValidator(simplePolicy)
		result, _ := validator.Validate(validating.CelParams{Object: simpleDeployment})
		decision := result.Decisions[0]
		expectedResult := k8sValidating.EvalDeny
		if expectedResult != decision.Evaluation {
			t.Errorf("decision evaluation is expected to be %s, but got %s", expectedResult, decision.Evaluation)
		}
	}

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

package kaptest
