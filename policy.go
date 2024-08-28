package validating

import (
	"context"
	"fmt"

	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/authentication/user"

	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/cel/environment"
)

// Interface to test Validating Admission Policy
type Validator interface {
	EvalMatchCondition(p CelParams) (*matchconditions.MatchResult, error)
	Validate(p CelParams) (*validating.ValidateResult, error)
}

type validator struct {
	validator validating.Validator
	policy    v1.ValidatingAdmissionPolicy
	matcher   matchconditions.Matcher
}

// Parameters that can be used in CEL expressions
type CelParams struct {
	Object       runtime.Object
	OldObject    runtime.Object
	ParamObj     runtime.Object
	NamespaceObj *corev1.Namespace
	UserInfo     user.Info
}

// NewValidator compiles the provided ValidatingAdmissionPolicy to evaluate CEL expressions
func NewValidator(policy v1.ValidatingAdmissionPolicy) *validator {
	v, m := compilePolicy(policy)
	return &validator{validator: v, policy: policy, matcher: m}
}

func compilePolicy(policy v1.ValidatingAdmissionPolicy) (validating.Validator, matchconditions.Matcher) {
	hasParam := false
	if policy.Spec.ParamKind != nil {
		hasParam = true
	}
	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: true, StrictCost: false}
	expressionOptionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false, StrictCost: false}
	failurePolicy := policy.Spec.FailurePolicy
	var matcher matchconditions.Matcher = nil
	matchConditions := policy.Spec.MatchConditions
	envTemplate, err := cel.NewCompositionEnv(cel.VariablesTypeName, environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), false))
	if err != nil {
		panic(err)
	}
	filterCompiler := cel.NewCompositedCompilerFromTemplate(envTemplate)
	filterCompiler.CompileAndStoreVariables(convertv1beta1Variables(policy.Spec.Variables), optionalVars, environment.StoredExpressions)

	if len(matchConditions) > 0 {
		matchExpressionAccessors := make([]cel.ExpressionAccessor, len(matchConditions))
		for i := range matchConditions {
			matchExpressionAccessors[i] = (*matchconditions.MatchCondition)(&matchConditions[i])
		}
		matcher = matchconditions.NewMatcher(filterCompiler.Compile(matchExpressionAccessors, optionalVars, environment.StoredExpressions), failurePolicy, "policy", "validate", policy.Name)
	}
	res := validating.NewValidator(
		filterCompiler.Compile(convertv1Validations(policy.Spec.Validations), optionalVars, environment.StoredExpressions),
		matcher,
		filterCompiler.Compile(convertv1AuditAnnotations(policy.Spec.AuditAnnotations), optionalVars, environment.StoredExpressions),
		filterCompiler.Compile(convertv1MessageExpressions(policy.Spec.Validations), expressionOptionalVars, environment.StoredExpressions),
		failurePolicy,
	)

	return res, matcher
}

func convertv1Validations(inputValidations []v1.Validation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		validation := validating.ValidationCondition{
			Expression: validation.Expression,
			Message:    validation.Message,
			Reason:     validation.Reason,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func convertv1MessageExpressions(inputValidations []v1.Validation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		if validation.MessageExpression != "" {
			condition := validating.MessageExpressionCondition{
				MessageExpression: validation.MessageExpression,
			}
			celExpressionAccessor[i] = &condition
		}
	}
	return celExpressionAccessor
}

func convertv1AuditAnnotations(inputValidations []v1.AuditAnnotation) []cel.ExpressionAccessor {
	celExpressionAccessor := make([]cel.ExpressionAccessor, len(inputValidations))
	for i, validation := range inputValidations {
		validation := validating.AuditAnnotationCondition{
			Key:             validation.Key,
			ValueExpression: validation.ValueExpression,
		}
		celExpressionAccessor[i] = &validation
	}
	return celExpressionAccessor
}

func convertv1beta1Variables(variables []v1.Variable) []cel.NamedExpressionAccessor {
	namedExpressions := make([]cel.NamedExpressionAccessor, len(variables))
	for i, variable := range variables {
		namedExpressions[i] = &validating.Variable{Name: variable.Name, Expression: variable.Expression}
	}
	return namedExpressions
}


// Evaluate policy's validations. ValidationResult contains the result of each validation
// (Admit, Deny, Error) and the reason if it is evaluated as Deny or Error
func (v *validator) Validate(p CelParams) (*validating.ValidateResult, error) {
	ctx := context.Background()
	nameWithGVK, err := getNameWithGVK(p)
	if err != nil {
		return nil, err
	}
	groupVersionResource := schema.GroupVersionResource{
		Group:    nameWithGVK.gvk.Group,
		Version:  nameWithGVK.gvk.Version,
		Resource: stubResource(),
	}
	matchedResource := groupVersionResource
	versionedAttribute := &admission.VersionedAttributes{
		Attributes: admission.NewAttributesRecord(
			p.Object,
			p.OldObject,
			nameWithGVK.gvk,
			nameWithGVK.namespace,
			nameWithGVK.name,
			groupVersionResource,
			stubSubResource(), stubAdmissionOperation(),
			stubOperationOptions(), stubIsDryRun(), p.UserInfo,
		),
		VersionedOldObject: p.OldObject,
		VersionedObject:    p.Object,
		VersionedKind:      nameWithGVK.gvk,
		Dirty:              false,
	}
	result := v.validator.Validate(
		ctx,
		matchedResource,
		versionedAttribute,
		p.ParamObj,
		p.NamespaceObj,
		stubRuntimeCELCostBudget(),
		stubAuthz(),
	)
	correctResult := correctValidateResult(result)
	return &correctResult, nil
}

type nameWithGVK struct {
	namespace string
	name      string
	gvk       schema.GroupVersionKind
}

func getNameWithGVK(p CelParams) (*nameWithGVK, error) {
	if p.Object == nil && p.OldObject == nil {
		return nil, fmt.Errorf("object or oldObject must be set")
	}
	obj := p.Object
	if obj == nil {
		obj = p.OldObject
	}
	namer := meta.NewAccessor()
	name, err := namer.Name(obj)
	if err != nil {
		return nil, fmt.Errorf("name is not valid: %v", err)
	}
	namespaceName, err := namer.Namespace(obj)
	if err != nil {
		return nil, fmt.Errorf("namespace is not valid: %v", err)
	}
	gvk := obj.GetObjectKind().GroupVersionKind()
	return &nameWithGVK{
		name:      name,
		namespace: namespaceName,
		gvk:       gvk,
	}, nil
}

// Workaround to handle the case where the evaluation is not set
// TODO remove this workaround after https://github.com/kubernetes/kubernetes/pull/126867 is released
func correctValidateResult(result validating.ValidateResult) validating.ValidateResult {
	for i, decision := range result.Decisions {
		if decision.Evaluation == "" {
			result.Decisions[i] = validating.PolicyDecision{
				Action:     decision.Action,
				Evaluation: validating.EvalDeny,
				Message:    decision.Message,
				Reason:     metav1.StatusReason(decision.Message),
				Elapsed:    decision.Elapsed,
			}
			decision.Evaluation = validating.EvalDeny
		}
	}
	return result
}
