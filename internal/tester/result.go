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

package tester

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/pfnet/kaptest"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

// testResult is the interface for the result of a test case.
type testResult interface {
	Pass() bool
	// String returns a human-readable string representation of the result.
	// If verbose is true, it includes the reason when the evaluation is not admitted.
	String(verbose bool) string
}

type vapEvalResult struct {
	Policy    string
	TestCase  VAPTestCase
	Decisions []validating.PolicyDecision
	Result    validating.PolicyDecisionEvaluation
}

var _ testResult = &vapEvalResult{}

func newVAPEvalResult(policy string, tc VAPTestCase, decisions []validating.PolicyDecision) *vapEvalResult {
	result := validating.EvalAdmit
	for _, d := range decisions {
		if d.Evaluation == validating.EvalDeny {
			result = validating.EvalDeny
		} else if d.Evaluation == validating.EvalError {
			result = validating.EvalError
			break
		}
	}

	return &vapEvalResult{
		Policy:    policy,
		TestCase:  tc,
		Decisions: decisions,
		Result:    result,
	}
}

func (r *vapEvalResult) Pass() bool {
	return string(r.Result) == string(r.TestCase.GetExpect())
}

func (r *vapEvalResult) String(verbose bool) string {
	summary := r.TestCase.SummaryLine(r.Pass(), r.Policy, string(r.Result))
	out := []string{summary}
	if !r.Pass() || verbose {
		for _, d := range r.Decisions {
			if d.Evaluation == validating.EvalDeny {
				out = append(out, fmt.Sprintf("--- DENY: reason %q, message %q", d.Reason, d.Message))
			} else if d.Evaluation == validating.EvalError {
				out = append(out, fmt.Sprintf("--- ERROR: reason %q, message %q", d.Reason, d.Message))
			}
		}
	}
	return strings.Join(out, "\n")
}

type mapEvalResult struct {
	policy         string
	testCase       MAPTestCase
	matchResults   []kaptest.MatchResult
	expectedObject runtime.Object
	mutatedObject  runtime.Object
}

var _ testResult = &vapEvalResult{}

func newMAPEvalResult(policy string, tc MAPTestCase, matchResults []kaptest.MatchResult, expectedObj, mutatedObj runtime.Object) *mapEvalResult {
	return &mapEvalResult{
		policy:         policy,
		testCase:       tc,
		matchResults:   matchResults,
		expectedObject: expectedObj,
		mutatedObject:  mutatedObj,
	}
}

func (r *mapEvalResult) Pass() bool {
	return equality.Semantic.DeepEqual(r.expectedObject, r.mutatedObject)
}

func (r *mapEvalResult) String(verbose bool) string {
	summary := r.testCase.SummaryLine(r.Pass(), r.policy, string(Mutate))
	out := []string{summary}
	if r.Pass() && !verbose {
		return strings.Join(out, "\n")
	}

	for _, h := range r.matchResults {
		o := fmt.Sprintf("--- Binding: %s ", h.Invocation.Binding.GetName())
		if h.Invocation.Param != nil {
			metaAcc, err := meta.Accessor(h.Invocation.Param)
			if err != nil {
				o += fmt.Sprintf("Param: failed to get param %v", err)
			} else {
				o += fmt.Sprintf("Param: %s %s", h.Invocation.Param.GetObjectKind().GroupVersionKind(), metaAcc.GetName())
			}
		} else {
			o += "Param: nil"
		}
		out = append(out, o)
	}
	if !r.Pass() {
		out = append(out, cmp.Diff(r.expectedObject, r.mutatedObject))
	}
	return strings.Join(out, "\n")
}

type policyNotFoundResult struct {
	Policy string
}

var _ testResult = &policyNotFoundResult{}

func newPolicyNotFoundResult(policy string) *policyNotFoundResult {
	return &policyNotFoundResult{
		Policy: policy,
	}
}

func (r *policyNotFoundResult) Pass() bool {
	return false
}

func (r *policyNotFoundResult) String(verbose bool) string {
	return fmt.Sprintf("FAIL: %s ==> POLICY NOT FOUND", r.Policy)
}

type setupErrorResult struct {
	Policy   string
	TestCase TestCase
	Errors   []error
}

var _ testResult = &setupErrorResult{}

func newSetupErrorResult(policy string, tc TestCase, errs []error) *setupErrorResult {
	return &setupErrorResult{
		Policy:   policy,
		TestCase: tc,
		Errors:   errs,
	}
}

func (r *setupErrorResult) Pass() bool {
	return false
}

func (r *setupErrorResult) String(verbose bool) string {
	summary := r.TestCase.SummaryLine(r.Pass(), r.Policy, "SETUP ERROR")
	out := []string{summary}
	for _, err := range r.Errors {
		out = append(out, fmt.Sprintf("--- ERROR: %v", err))
	}
	return strings.Join(out, "\n")
}

type policyNotMatchConditionResult struct {
	Policy              string
	TestCase            TestCase
	FailedConditionName string
}

var _ testResult = &policyNotMatchConditionResult{}

func newPolicyNotMatchConditionResult(policy string, tc TestCase, failedConditionName string) *policyNotMatchConditionResult {
	return &policyNotMatchConditionResult{
		Policy:              policy,
		TestCase:            tc,
		FailedConditionName: failedConditionName,
	}
}

func (r *policyNotMatchConditionResult) Pass() bool {
	return r.TestCase.GetExpect() == Skip
}

func (r *policyNotMatchConditionResult) String(verbose bool) string {
	summary := r.TestCase.SummaryLine(r.Pass(), r.Policy, "SKIP")
	out := []string{summary}
	if !r.Pass() || verbose {
		out = append(out, fmt.Sprintf("--- NOT MATCH: condition-name %q", r.FailedConditionName))
	}

	return strings.Join(out, "\n")
}

type policyEvalErrorResult struct {
	Policy   string
	TestCase TestCase
	Errors   []error
}

var _ testResult = &policyEvalErrorResult{}

func newPolicyEvalErrorResult(policy string, tc TestCase, errs []error) *policyEvalErrorResult {
	return &policyEvalErrorResult{
		Policy:   policy,
		TestCase: tc,
		Errors:   errs,
	}
}

func (r *policyEvalErrorResult) Pass() bool {
	return r.TestCase.GetExpect() == Error
}

func (r *policyEvalErrorResult) String(verbose bool) string {
	summary := r.TestCase.SummaryLine(r.Pass(), r.Policy, "ERROR")
	out := []string{summary}
	if !r.Pass() || verbose {
		for _, err := range r.Errors {
			out = append(out, fmt.Sprintf("--- ERROR: %v", err))
		}
	}

	return strings.Join(out, "\n")
}

type testResultSummary struct {
	manifestPath string
	pass         int
	fail         int
	message      string
}

var _ testResult = &testResultSummary{}

func (s *testResultSummary) Pass() bool {
	return s.fail == 0
}

func (s *testResultSummary) String(verbose bool) string {
	out := []string{
		fmt.Sprintf("[%s]", s.manifestPath),
		s.message,
		fmt.Sprintf("Total: %d, Pass: %d, Fail: %d\n", s.pass+s.fail, s.pass, s.fail),
	}
	return strings.Join(out, "\n")
}

func summarize(manifestPath string, results []testResult, verbose bool) testResultSummary {
	summary := testResultSummary{
		manifestPath: manifestPath,
	}
	out := []string{}
	for _, r := range results {
		if r.Pass() {
			summary.pass++
		} else {
			summary.fail++
		}
		out = append(out, r.String(verbose))
	}
	summary.message = strings.Join(out, "\n")

	return summary
}
