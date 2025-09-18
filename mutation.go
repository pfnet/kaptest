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
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/api/admissionregistration/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	plugincel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/admission/plugin/policy/generic"
	"k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating"
	"k8s.io/apiserver/pkg/admission/plugin/policy/mutating/patch"
	webhookgeneric "k8s.io/apiserver/pkg/admission/plugin/webhook/generic"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/environment"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/openapi/openapitest"
	"k8s.io/utils/ptr"
)

type MutatorInterface interface {
	EvalMatchCondition(p MutationParams) ([]MatchResult, error)
	Mutate(p MutationParams) (runtime.Object, error)
}

type Mutator struct {
	policy    *v1alpha1.MutatingAdmissionPolicy
	evaluator mutating.PolicyEvaluator
}

var _ MutatorInterface = &Mutator{}

type MutationParams = ValidationParams

func (p MutationParams) GetGVK() schema.GroupVersionKind {
	op := p.Operation()
	switch p.Operation() {
	case admission.Create, admission.Update:
		return p.Object.GetObjectKind().GroupVersionKind()
	case admission.Delete:
		return p.OldObject.GetObjectKind().GroupVersionKind()
	default:
		panic(fmt.Errorf("unexpected operation: %v", op))
	}
}

func (p MutationParams) VersionedAttributes() (*admission.VersionedAttributes, error) {
	metaAcc, err := meta.Accessor(p.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to crate meta.Accessor: %w", err)
	}

	gvk := p.Object.GetObjectKind().GroupVersionKind()

	// TODO: fix this
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)

	// TODO: fill subResources
	// TODO: fill operationOptions
	attrs := admission.NewAttributesRecord(p.Object, p.OldObject, p.GetGVK(), metaAcc.GetNamespace(), metaAcc.GetName(), gvr, "", p.Operation(), nil, true, p.UserInfo)
	return &admission.VersionedAttributes{
		Attributes:         attrs,
		VersionedKind:      gvk,
		VersionedObject:    p.Object.DeepCopyObject(),
		VersionedOldObject: p.OldObject,
	}, nil
}

func NewMutator(policy *v1alpha1.MutatingAdmissionPolicy) (*Mutator, error) {
	evaluator := compileMutatitionAddmissionPolicy(policy)
	if evaluator.Error != nil {
		return nil, evaluator.Error
	}

	return &Mutator{
		policy:    policy,
		evaluator: evaluator,
	}, nil
}

type mutatorContext struct {
	tcm             patch.TypeConverterManager
	auth            authorizer.Authorizer
	objInterface    admission.ObjectInterfaces
	matcher         *matching.Matcher
	client          *fake.Clientset
	informerFactory informers.SharedInformerFactory
}

func newMutatorContext(ctx context.Context) (*mutatorContext, error) {
	// Prepare TypeConvertManager
	// TODO: support CRDs
	tcm := patch.NewTypeConverterManager(nil, openapitest.NewEmbeddedFileClient())
	go tcm.Run(ctx)

	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, time.Second, false, func(context.Context) (done bool, err error) {
		// wait for schemes become ready
		converter := tcm.GetTypeConverter(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		return converter != nil, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to prepare TypeConvertManager: %w", err)
	}

	// Prepare Authorization
	authorizer := stubAuthz()

	// Prepare ObjectInterface
	// TODO: support more schemes?
	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to register scheme: %w", err)
	}

	// TODO: support DefaultingFunc with schema.AddTypeDefaultingFunc()?
	// What will happen when mutating with the default values?
	objInterface := admission.NewObjectInterfacesFromScheme(scheme)

	// Prepare Client
	client := fake.NewClientset()

	// Prepare matcher
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	matcher := matching.NewMatcher(informerFactory.Core().V1().Namespaces().Lister(), client)

	return &mutatorContext{
		tcm:             tcm,
		auth:            authorizer,
		objInterface:    objInterface,
		matcher:         matcher,
		client:          client,
		informerFactory: informerFactory,
	}, nil
}

func (mc *mutatorContext) addObjectAndEnsureSynced(ctx context.Context, obj runtime.Object) error {
	err := mc.client.Tracker().Add(obj)
	if err != nil {
		return fmt.Errorf("failed to add object: %w", err)
	}
	// TODO: better GVR handling
	gvr, _ := meta.UnsafeGuessKindToResource(obj.GetObjectKind().GroupVersionKind())
	informer, err := mc.informerFactory.ForResource(gvr)
	if err != nil {
		return fmt.Errorf("failed to get informer: %w", err)
	}
	acc, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("failed to get accessor: %w", err)
	}
	err = wait.PollUntilContextTimeout(ctx, 1*time.Millisecond, 1*time.Second, true, func(context context.Context) (done bool, err error) {
		namespace := acc.GetNamespace()
		if namespace != "" {
			_, err = informer.Lister().ByNamespace(namespace).Get(acc.GetName())
		} else {
			_, err = informer.Lister().Get(acc.GetName())
		}
		return err == nil, nil
	})
	if err != nil {
		return fmt.Errorf("poll timeout: %w", err)
	}

	return nil
}

type MutatingPolicyInvocation = generic.PolicyInvocation[*mutating.Policy, *mutating.PolicyBinding, mutating.PolicyEvaluator]

type MatchResult struct {
	Result     matchconditions.MatchResult
	Invocation MutatingPolicyInvocation
}
type dispatchRecoder struct {
	attrs        *admission.VersionedAttributes
	matchResults []MatchResult
}

func newDispatchRecoder(p MutationParams) (*dispatchRecoder, error) {
	attrs, err := p.VersionedAttributes()
	if err != nil {
		return nil, fmt.Errorf("failed to get versionedAttributes: %w", err)
	}
	return &dispatchRecoder{
		attrs:        attrs,
		matchResults: []MatchResult{},
	}, nil
}

func (d *dispatchRecoder) dispatchInvocations(
	ctx context.Context,
	a admission.Attributes,
	o admission.ObjectInterfaces,
	versionedAttributes webhookgeneric.VersionedAttributeAccessor,
	invocations []MutatingPolicyInvocation,
) ([]generic.PolicyError, *k8serrors.StatusError) {
	// Logic comes from https://github.com/kubernetes/apiserver/blob/v0.32.1/pkg/admission/plugin/policy/mutating/dispatcher.go#L151
	for idx := range invocations {
		h := invocations[idx]
		if h.Evaluator.Matcher == nil {
			d.matchResults = append(d.matchResults, MatchResult{
				Result: matchconditions.MatchResult{
					Matches: true,
				},
				Invocation: h,
			})
			continue
		}
		m := h.Evaluator.Matcher.Match(ctx, d.attrs, h.Param, stubAuthz())
		d.matchResults = append(d.matchResults, MatchResult{
			Result:     m,
			Invocation: h,
		})
	}
	return nil, nil
}

// EvalMatchCondition returns matched param objects.
func (m *Mutator) EvalMatchCondition(p MutationParams) ([]MatchResult, error) {
	recoder, err := newDispatchRecoder(p)
	if err != nil {
		return nil, fmt.Errorf("failed to create dispatchRecorder: %w", err)
	}
	_, err = m.dispatchImpl(p, func(mCtx *mutatorContext) generic.Dispatcher[mutating.PolicyHook] {
		return generic.NewPolicyDispatcher(
			mutating.NewMutatingAdmissionPolicyAccessor,
			mutating.NewMutatingAdmissionPolicyBindingAccessor,
			mCtx.matcher,
			recoder.dispatchInvocations,
		)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch: %w", err)
	}

	return recoder.matchResults, nil
}

func (m *Mutator) Mutate(p MutationParams) (runtime.Object, error) {
	return m.dispatchImpl(p, func(mCtx *mutatorContext) generic.Dispatcher[mutating.PolicyHook] {
		return mutating.NewDispatcher(mCtx.auth, mCtx.matcher, mCtx.tcm)
	})
}

func (m *Mutator) dispatchImpl(p MutationParams, dispatcherFactory func(mCtx *mutatorContext) generic.Dispatcher[mutating.PolicyHook]) (runtime.Object, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mCtx, err := newMutatorContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize mutatorContext: %w", err)
	}

	binding := &v1alpha1.MutatingAdmissionPolicyBinding{
		Spec: v1alpha1.MutatingAdmissionPolicyBindingSpec{
			ParamRef: &v1alpha1.ParamRef{},
			MatchResources: &v1alpha1.MatchResources{
				MatchPolicy:       ptr.To(v1alpha1.Equivalent),
				ObjectSelector:    &metav1.LabelSelector{},
				NamespaceSelector: &metav1.LabelSelector{},
			},
		},
	}

	hook := mutating.PolicyHook{
		Policy:    m.policy,
		Evaluator: m.evaluator,
		Bindings:  []*mutating.PolicyBinding{binding},
	}

	if m.policy.Spec.ParamKind != nil {
		// TODO: more better handling
		paramGV, err := schema.ParseGroupVersion(m.policy.Spec.ParamKind.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to parse ParamKind.APIVersion %v: %w", m.policy.Spec.ParamKind.APIVersion, err)
		}
		paramGVR, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
			Group:   paramGV.Group,
			Version: paramGV.Version,
			Kind:    m.policy.Spec.ParamKind.Kind,
		})
		paramInformer, err := mCtx.informerFactory.ForResource(paramGVR)
		if err != nil {
			return nil, fmt.Errorf("failed to create informer for params: %w", err)
		}
		hook.ParamInformer = paramInformer

		// TODO: Configure this correctly
		// This filters parameters based on the paramRef's namespace
		hook.ParamScope = namespaceParamScope{}

		metaAcc, err := meta.Accessor(p.ParamObj)
		if err != nil {
			return nil, fmt.Errorf("failed to access paramObj: %w", err)
		}
		binding.Spec.ParamRef.Name = metaAcc.GetName()
		binding.Spec.ParamRef.Namespace = metaAcc.GetNamespace()
	}

	// Start informers
	mCtx.informerFactory.WaitForCacheSync(ctx.Done())
	mCtx.informerFactory.Start(ctx.Done())

	objs := []runtime.Object{}
	if p.ParamObj != nil {
		objs = append(objs, p.ParamObj)
	}
	if p.NamespaceObj != nil {
		objs = append(objs, p.NamespaceObj)
	}

	// This includes parameters represented in configmaps.
	for _, o := range objs {
		err := mCtx.addObjectAndEnsureSynced(ctx, o)
		if err != nil {
			return nil, fmt.Errorf("failed to add object: %w", err)
		}
	}

	// Prepare dispatcher
	dispatcher := dispatcherFactory(mCtx)
	err = dispatcher.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start dispatcher: %w", err)
	}

	attrs, err := p.VersionedAttributes()
	if err != nil {
		return nil, fmt.Errorf("failed to get VersionedAttributes for object: %w", err)
	}
	err = dispatcher.Dispatch(ctx, attrs, mCtx.objInterface, []mutating.PolicyHook{hook})
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch mutating request: %w", err)
	}

	// Write-back GVK
	attrs.VersionedObject.GetObjectKind().SetGroupVersionKind(p.Object.GetObjectKind().GroupVersionKind())
	return attrs.VersionedObject, err
}

type namespaceParamScope struct{}

func (n namespaceParamScope) Name() meta.RESTScopeName {
	return meta.RESTScopeNameNamespace
}

var _ meta.RESTScope = namespaceParamScope{}

// Original: https://github.com/kubernetes/apiserver/blob/v0.32.1/pkg/admission/plugin/policy/mutating/compilation.go
func compileMutatitionAddmissionPolicy(policy *mutating.Policy) mutating.PolicyEvaluator {
	opts := plugincel.OptionalVariableDeclarations{HasParams: policy.Spec.ParamKind != nil, StrictCost: true, HasAuthorizer: true}
	compiler, err := plugincel.NewCompositedCompiler(environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), true))
	if err != nil {
		return mutating.PolicyEvaluator{Error: &apiservercel.Error{
			Type:   apiservercel.ErrorTypeInternal,
			Detail: fmt.Sprintf("failed to initialize CEL compiler: %v", err),
		}}
	}

	// Compile and store variables
	compiler.CompileAndStoreVariables(convertv1alpha1Variables(policy.Spec.Variables), opts, environment.StoredExpressions)

	// Compile matchers
	var matcher matchconditions.Matcher = nil
	matchConditions := policy.Spec.MatchConditions
	if len(matchConditions) > 0 {
		matchExpressionAccessors := make([]plugincel.ExpressionAccessor, len(matchConditions))
		for i := range matchConditions {
			matchExpressionAccessors[i] = (*matchconditions.MatchCondition)(&matchConditions[i])
		}
		matcher = matchconditions.NewMatcher(compiler.CompileCondition(matchExpressionAccessors, opts, environment.StoredExpressions), toV1FailurePolicy(policy.Spec.FailurePolicy), "policy", "validate", policy.Name)
	}

	// Compiler patchers
	var patchers []patch.Patcher
	patchOptions := opts
	patchOptions.HasPatchTypes = true
	for _, m := range policy.Spec.Mutations {
		switch m.PatchType {
		case v1alpha1.PatchTypeJSONPatch:
			if m.JSONPatch != nil {
				accessor := &patch.JSONPatchCondition{Expression: m.JSONPatch.Expression}
				compileResult := compiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewJSONPatcher(compileResult))
			}
		case v1alpha1.PatchTypeApplyConfiguration:
			if m.ApplyConfiguration != nil {
				accessor := &patch.ApplyConfigurationCondition{Expression: m.ApplyConfiguration.Expression}
				compileResult := compiler.CompileMutatingEvaluator(accessor, patchOptions, environment.StoredExpressions)
				patchers = append(patchers, patch.NewApplyConfigurationPatcher(compileResult))
			}
		}
	}

	return mutating.PolicyEvaluator{Matcher: matcher, Mutators: patchers, CompositionEnv: compiler.CompositionEnv}
}

// Original: https://github.com/kubernetes/apiserver/blob/v0.32.1/pkg/admission/plugin/policy/mutating/plugin.go#L145-L151
func convertv1alpha1Variables(variables []v1alpha1.Variable) []plugincel.NamedExpressionAccessor {
	namedExpressions := make([]plugincel.NamedExpressionAccessor, len(variables))
	for i, variable := range variables {
		namedExpressions[i] = &mutating.Variable{Name: variable.Name, Expression: variable.Expression}
	}
	return namedExpressions
}

// Original: https://github.com/kubernetes/apiserver/blob/v0.32.1/pkg/admission/plugin/policy/mutating/accessor.go#L69-L75
func toV1FailurePolicy(failurePolicy *v1alpha1.FailurePolicyType) *v1.FailurePolicyType {
	if failurePolicy == nil {
		return nil
	}
	fp := v1.FailurePolicyType(*failurePolicy)
	return &fp
}
