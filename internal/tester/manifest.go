package tester

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	"k8s.io/apiserver/pkg/authentication/user"
)

type TestManifests struct {
	ValidatingAdmissionPolicies []string               `yaml:"validatingAdmissionPolicies,omitempty"`
	Resources                   []string               `yaml:"resources,omitempty"`
	Params                      []string               `yaml:"params,omitempty"`
	Namespaces                  []string               `yaml:"namespaces,omitempty"`
	TestSuites                  []TestsForSinglePolicy `yaml:"testSuites,omitempty"`
}

type TestsForSinglePolicy struct {
	Policy string     `yaml:"policy"`
	Tests  []TestCase `yaml:"tests"`
}

type TestCase struct {
	Object    NameWithGVK                         `yaml:"object,omitempty"`
	OldObject NameWithGVK                         `yaml:"oldObject,omitempty"`
	Param     NamespacedName                      `yaml:"param,omitempty"`
	Expect    validating.PolicyDecisionEvaluation `yaml:"expect,omitempty"`
	UserInfo  UserInfo                            `yaml:"userInfo,omitempty"`
	// TODO: Support message test
	// Message   string                              `yaml:"message"`
}

type GVK struct {
	Group   string `yaml:"group,omitempty"`
	Version string `yaml:"version,omitempty"`
	Kind    string `yaml:"kind"`
}

type NamespacedName struct {
	Namespace string `yaml:"namespace,omitempty"`
	Name      string `yaml:"name"`
}

func (n NamespacedName) IsValid() bool {
	return n.Name != ""
}

func (n NamespacedName) String() string {
	if n.Namespace != "" && n.Name != "" {
		return n.Namespace + "/" + n.Name
	}
	return n.Name
}

type NameWithGVK struct {
	GVK            `yaml:",inline"`
	NamespacedName `yaml:",inline"`
}

func (n NameWithGVK) String() string {
	return n.Kind + ":" + n.NamespacedName.String()
}

func (n NameWithGVK) IsValid() bool {
	return n.Name != "" && n.Kind != ""
}

func (n NameWithGVK) Match(o NameWithGVK) bool {
	if !n.IsValid() || !o.IsValid() {
		return false
	}
	if n.Name != o.Name {
		return false
	}
	if n.Kind != o.Kind {
		return false
	}
	// Check namespace only if either of them has namespace
	if (n.Namespace != "" || o.Namespace != "") && n.Namespace != o.Namespace {
		return false
	}
	// If group is empty, it is considered as a match
	if n.Group == "" || o.Group == "" {
		return true
	}
	if n.Group != o.Group {
		return false
	}
	// If version is empty, it is considered as a match
	if n.Version == "" || o.Version == "" {
		return true
	}
	if n.Version != o.Version {
		return false
	}
	return true
}

func NewNameWithGVKFromObj(obj *unstructured.Unstructured) NameWithGVK {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return NameWithGVK{
		GVK: GVK{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		NamespacedName: NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}
}

func NewNameWithGVK(gvk schema.GroupVersionKind, namespacedName NamespacedName) NameWithGVK {
	return NameWithGVK{
		GVK: GVK{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		NamespacedName: namespacedName,
	}
}

type UserInfo struct {
	Name   string   `yaml:"name"`
	Groups []string `yaml:"groups"`
	Extra  map[string][]string
}

func NewUserInfo(u UserInfo) user.DefaultInfo {
	return user.DefaultInfo{
		Name:   u.Name,
		Groups: u.Groups,
		Extra:  u.Extra,
	}
}
