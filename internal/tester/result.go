package tester

import (
	"fmt"
	"strings"

	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
)

// TestResult is the interface for the result of a test case.
type TestResult interface {
	Pass() bool
	// String returns a human-readable string representation of the result.
	// If verbose is true, it includes the reason when the evaluation is not admitted.
	String(verbose bool) string
}

var _ TestResult = &policyEvalResult{}

type policyEvalResult struct {
	Policy    string
	TestCase  TestCase
	Decisions []validating.PolicyDecision
	Result    validating.PolicyDecisionEvaluation
}

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
	return r.Result == r.TestCase.Expect
}

func (r *policyEvalResult) String(verbose bool) string {
	var summary string
	if r.Pass() {
		summary = "PASS"
	} else {
		summary = "FAIL"
	}

	summary += fmt.Sprintf(": %s", r.Policy)
	if r.TestCase.Object.IsValid() && r.TestCase.OldObject.IsValid() {
		summary += fmt.Sprintf(" - %s -> %s ", r.TestCase.Object.String(), r.TestCase.OldObject.NamespacedName.String())
	} else if r.TestCase.Object.IsValid() {
		summary += fmt.Sprintf(" - %s", r.TestCase.Object.String())
	} else if r.TestCase.OldObject.IsValid() {
		summary += fmt.Sprintf(" - %s", r.TestCase.OldObject.String())
	}
	if r.TestCase.Param.IsValid() {
		summary += fmt.Sprintf(" (Param: %s)", r.TestCase.Param.String())
	}
	summary += fmt.Sprintf(" - %s ==> %s", strings.ToUpper(string(r.TestCase.Expect)), strings.ToUpper(string(r.Result)))

	if !verbose {
		return summary
	}

	out := []string{summary}
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
	return strings.Join(out, "\n")
}

func Summarize(results []TestResult, verbose bool) (string, bool) {
	passCount := 0
	failCount := 0
	out := []string{}
	for _, r := range results {
		if r.Pass() {
			passCount++
		} else {
			failCount++
		}
	}
	for _, r := range results {
		out = append(out, r.String(verbose))
	}
	out = append(out, fmt.Sprintf("\nTotal: %d, Pass: %d, Fail: %d", len(results), passCount, failCount))

	return strings.Join(out, "\n"), failCount == 0
}

type PolicyNotFoundResult struct {
	Policy string
}

var _ TestResult = &PolicyNotFoundResult{}

func NewPolicyNotFoundResult(policy string) *PolicyNotFoundResult {
	return &PolicyNotFoundResult{
		Policy: policy,
	}
}

func (r *PolicyNotFoundResult) Pass() bool {
	return false
}

func (r *PolicyNotFoundResult) String(verbose bool) string {
	return fmt.Sprintf("FAIL: %s ==> POLICY NOT FOUND", r.Policy)
}

type PolicyEvalErrorResult struct {
	Policy   string
	TestCase TestCase
	Errors   []error
}

var _ TestResult = &PolicyEvalErrorResult{}

func NewPolicyEvalErrorResult(policy string, tc TestCase, errs []error) *PolicyEvalErrorResult {
	return &PolicyEvalErrorResult{
		Policy:   policy,
		TestCase: tc,
		Errors:   errs,
	}
}

func (r *PolicyEvalErrorResult) Pass() bool {
	return false
}

func (r *PolicyEvalErrorResult) String(verbose bool) string {
	summary := fmt.Sprintf("FAIL: %s", r.Policy)
	if r.TestCase.Object.IsValid() && r.TestCase.OldObject.IsValid() {
		summary += fmt.Sprintf(" - %s -> %s ", r.TestCase.Object.String(), r.TestCase.OldObject.NamespacedName.String())
	} else if r.TestCase.Object.IsValid() {
		summary += fmt.Sprintf(" - %s", r.TestCase.Object.String())
	} else if r.TestCase.OldObject.IsValid() {
		summary += fmt.Sprintf(" - %s", r.TestCase.OldObject.String())
	}
	if r.TestCase.Param.IsValid() {
		summary += fmt.Sprintf(" (Param: %s)", r.TestCase.Param.String())
	}
	summary += fmt.Sprintf(" - %s ==> %s", strings.ToUpper(string(r.TestCase.Expect)), "SETUP ERROR")

	out := []string{summary}
	for _, err := range r.Errors {
		out = append(out, fmt.Sprintf("--- ERROR: %v", err))
	}
	return strings.Join(out, "\n")
}
