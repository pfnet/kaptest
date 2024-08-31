package kaptest

import (
	"context"
	"fmt"

	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/authentication/user"

	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/cel/environment"
)

// Validator is an interface to evaluate ValidatingAdmissionPolicy.
type Validator interface {
	EvalMatchCondition(p ValidationParams) (*matchconditions.MatchResult, error)
	Validate(p ValidationParams) (*validating.ValidateResult, error)
}

type validator struct {
	validator validating.Validator
	policy    v1.ValidatingAdmissionPolicy
	matcher   matchconditions.Matcher
}

// ValidationParams contains the parameters required to evaluate a ValidatingAdmissionPolicy.
type ValidationParams struct {
	Object       runtime.Object
	OldObject    runtime.Object
	ParamObj     runtime.Object
	NamespaceObj *corev1.Namespace
	UserInfo     user.Info
}

func (p ValidationParams) Operation() admission.Operation {
	if p.Object != nil && p.OldObject != nil {
		return admission.Update
	}
	if p.Object != nil {
		return admission.Create
	}
	return admission.Delete
}

// NewValidator compiles the provided ValidatingAdmissionPolicy and generates Validator.
func NewValidator(policy v1.ValidatingAdmissionPolicy) *validator {
	v, m := compilePolicy(policy)
	return &validator{validator: v, policy: policy, matcher: m}
}

// Original: https://github.com/kubernetes/kubernetes/blob/8bd6c10ba5833369fb6582587b77de8f8b51c371/staging/src/k8s.io/apiserver/pkg/admission/plugin/policy/validating/plugin.go#L121-L157
func compilePolicy(policy v1.ValidatingAdmissionPolicy) (validating.Validator, matchconditions.Matcher) {
	hasParam := false
	if policy.Spec.ParamKind != nil {
		hasParam = true
	}
	// NOTE: StrictCost option is disabled for now.
	optionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: true, StrictCost: false}
	expressionOptionalVars := cel.OptionalVariableDeclarations{HasParams: hasParam, HasAuthorizer: false, StrictCost: false}
	failurePolicy := policy.Spec.FailurePolicy
	var matcher matchconditions.Matcher = nil
	matchConditions := policy.Spec.MatchConditions
	compositionEnvTemplate, err := cel.NewCompositionEnv(cel.VariablesTypeName, environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), false))
	if err != nil {
		panic(err)
	}
	filterCompiler := cel.NewCompositedCompilerFromTemplate(compositionEnvTemplate)
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

// EvalMatchCondition evaluates ValidatingAdmissionPolicies' match conditions.
// It returns the result of the matchCondition evaluation to tell the caller which one is evaluated as 'false'.
// This is a hack to be able to check the name of failed expressions in matchCondition.
//
// TODO: Remove this func after k/k's Validate func outputs the name of the failed matchCondition.
func (v *validator) EvalMatchCondition(p ValidationParams) (*matchconditions.MatchResult, error) {
	if v.matcher == nil {
		return nil, fmt.Errorf("match condition is not defined")
	}
	ctx := context.Background()
	versionedAttribute, _ := makeVersionedAttribute(p)
	matchResults := v.matcher.Match(ctx, versionedAttribute, p.ParamObj, stubAuthz())
	return &matchResults, nil
}

// Validate evaluates ValidationAdmissionPolicies' validations.
// ValidationResult contains the result of each validation(Admit, Deny, Error)
// and the reason if it is evaluated as Deny or Error.
func (v *validator) Validate(p ValidationParams) (*validating.ValidateResult, error) {
	ctx := context.Background()
	versionedAttribute, matchedResource := makeVersionedAttribute(p)
	result := v.validator.Validate(
		ctx,
		matchedResource,
		versionedAttribute,
		p.ParamObj,
		p.NamespaceObj,
		celconfig.RuntimeCELCostBudget,
		// Inject stub authorizer since this testing tool focuses on the validation logic.
		stubAuthz(),
	)
	correctResult := correctValidateResult(result)
	return &correctResult, nil
}

func makeVersionedAttribute(p ValidationParams) (*admission.VersionedAttributes, schema.GroupVersionResource) {
	nameWithGVK, err := getNameWithGVK(p)
	if err != nil {
		return nil, schema.GroupVersionResource{}
	}
	groupVersionResource := schema.GroupVersionResource{
		Group:   nameWithGVK.gvk.Group,
		Version: nameWithGVK.gvk.Version,
		// NOTE: GVR.Resource is not populated
		Resource: "",
	}
	return &admission.VersionedAttributes{
		Attributes: admission.NewAttributesRecord(
			p.Object,
			p.OldObject,
			nameWithGVK.gvk,
			nameWithGVK.namespace,
			nameWithGVK.name,
			groupVersionResource,
			// NOTE: subResource is not populated
			"", // subResource
			p.Operation(),
			// NOTE: operationOptions is not populated
			nil, // operationOptions
			// NOTE: dryRun is always true
			true, // dryRun
			p.UserInfo,
		),
		VersionedOldObject: p.OldObject,
		VersionedObject:    p.Object,
		VersionedKind:      nameWithGVK.gvk,
		Dirty:              false,
	}, groupVersionResource
}

type nameWithGVK struct {
	namespace string
	name      string
	gvk       schema.GroupVersionKind
}

func getNameWithGVK(p ValidationParams) (*nameWithGVK, error) {
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

// Workaround to handle the case where the evaluation is not set.
// TODO: remove this workaround after https://github.com/kubernetes/kubernetes/pull/126867 is released
func correctValidateResult(result validating.ValidateResult) validating.ValidateResult {
	for i, decision := range result.Decisions {
		if decision.Evaluation == "" {
			decision.Evaluation = validating.EvalDeny
			result.Decisions[i] = decision
		}
	}
	return result
}
