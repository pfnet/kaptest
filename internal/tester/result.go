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

	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

// testResult is the interface for the result of a test case.
type testResult interface {
	Pass() bool
	// String returns a human-readable string representation of the result.
	// If verbose is true, it includes the reason when the evaluation is not admitted.
	String(verbose bool) string
}

func summaryLine(pass bool, policy string, testCase TestCase, result string) string {
	var summary string
	if pass {
		summary = "PASS"
	} else {
		summary = "FAIL"
	}

	summary += fmt.Sprintf(": %s", policy)
	if testCase.Object.IsValid() && testCase.OldObject.IsValid() { //nolint:gocritic
		summary += fmt.Sprintf(" - (UPDATE) %s -> %s", testCase.OldObject.String(), testCase.Object.NamespacedName.String())
	} else if testCase.Object.IsValid() {
		summary += fmt.Sprintf(" - (CREATE) %s", testCase.Object.String())
	} else if testCase.OldObject.IsValid() {
		summary += fmt.Sprintf(" - (DELETE) %s", testCase.OldObject.String())
	}
	if testCase.Param.IsValid() {
		summary += fmt.Sprintf(" (Param: %s)", testCase.Param.String())
	}
	summary += fmt.Sprintf(" - %s ==> %s", strings.ToUpper(string(testCase.Expect)), result)
	return summary
}

type policyEvalResult struct {
	Policy    string
	TestCase  TestCase
	Decisions []validating.PolicyDecision
	Result    validating.PolicyDecisionEvaluation
}

var _ testResult = &policyEvalResult{}

func newPolicyEvalResult(policy string, tc TestCase, decisions []validating.PolicyDecision) *policyEvalResult {
	result := validating.EvalAdmit
	for _, d := range decisions {
		if d.Evaluation == validating.EvalDeny {
			result = validating.EvalDeny
		} else if d.Evaluation == validating.EvalError {
			result = validating.EvalError
			break
		}
	}

	return &policyEvalResult{
		Policy:    policy,
		TestCase:  tc,
		Decisions: decisions,
		Result:    result,
	}
}

func (r *policyEvalResult) Pass() bool {
	return string(r.Result) == string(r.TestCase.Expect)
}

func (r *policyEvalResult) String(verbose bool) string {
	summary := summaryLine(r.Pass(), r.Policy, r.TestCase, string(r.Result))
	out := []string{summary}
	if !r.Pass() || verbose {
		for _, d := range r.Decisions {
			// Workaround to handle the case where the evaluation is not set
			// TODO remove this workaround after htcps://github.com/kubernetes/kubernetes/pull/126867 is released
			if d.Evaluation == "" {
				d.Evaluation = validating.EvalDeny
			}
			if d.Evaluation == validating.EvalDeny {
				out = append(out, fmt.Sprintf("--- DENY: reason %q, message %q", d.Reason, d.Message))
			} else if d.Evaluation == validating.EvalError {
				out = append(out, fmt.Sprintf("--- ERROR: reason %q, message %q", d.Reason, d.Message))
			}
		}
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
	summary := summaryLine(r.Pass(), r.Policy, r.TestCase, "SETUP ERROR")
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
	return r.TestCase.Expect == Skip
}

func (r *policyNotMatchConditionResult) String(verbose bool) string {
	summary := summaryLine(r.Pass(), r.Policy, r.TestCase, "SKIP")
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
	return r.TestCase.Expect == Error
}

func (r *policyEvalErrorResult) String(verbose bool) string {
	summary := summaryLine(r.Pass(), r.Policy, r.TestCase, "ERROR")
	out := []string{summary}
	if !r.Pass() || verbose {
		for _, err := range r.Errors {
			out = append(out, fmt.Sprintf("--- ERROR: %v", err))
		}
	}

	return strings.Join(out, "\n")
}

type policyEvalFatalErrorResult struct {
	Policy   string
	TestCase TestCase
	Errors   []error
}

var _ testResult = &policyEvalFatalErrorResult{}

func newPolicyEvalFatalErrorResult(policy string, tc TestCase, errs []error) *policyEvalFatalErrorResult {
	return &policyEvalFatalErrorResult{
		Policy:   policy,
		TestCase: tc,
		Errors:   errs,
	}
}

func (r *policyEvalFatalErrorResult) Pass() bool {
	return false
}

func (r *policyEvalFatalErrorResult) String(verbose bool) string {
	summary := summaryLine(r.Pass(), r.Policy, r.TestCase, "FATAL ERROR")
	out := []string{summary}
	for _, err := range r.Errors {
		out = append(out, fmt.Sprintf("--- ERROR: %v", err))
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
