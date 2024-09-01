package tester

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
)

// TestManifests is a struct to represent the whole test manifest file.
type TestManifests struct {
	ValidatingAdmissionPolicies []string               `yaml:"validatingAdmissionPolicies,omitempty"`
	Resources                   []string               `yaml:"resources,omitempty"`
	TestSuites                  []TestsForSinglePolicy `yaml:"testSuites,omitempty"`
}

func (t TestManifests) IsValid() (bool, string) {
	if len(t.ValidatingAdmissionPolicies) == 0 {
		return false, "at least one validatingAdmissionPolicies is required"
	}
	if len(t.Resources) == 0 {
		return false, "at least one resources is required"
	}
	if len(t.TestSuites) == 0 {
		return false, "at least one testSuites is required"
	}
	return true, ""
}

// TestsForSinglePolicy is a struct to aggregate multiple test cases for a single policy.
type TestsForSinglePolicy struct {
	Policy string     `yaml:"policy"`
	Tests  []TestCase `yaml:"tests"`
}

type PolicyDecisionExpect string

const (
	Admit PolicyDecisionExpect = "admit"
	Deny  PolicyDecisionExpect = "deny"
	Error PolicyDecisionExpect = "error"
	Skip  PolicyDecisionExpect = "skip"
)

// TestCase is a struct to represent a single test case.
type TestCase struct {
	Object    NameWithGVK          `yaml:"object,omitempty"`
	OldObject NameWithGVK          `yaml:"oldObject,omitempty"`
	Param     NamespacedName       `yaml:"param,omitempty"`
	Expect    PolicyDecisionExpect `yaml:"expect,omitempty"`
	UserInfo  UserInfo             `yaml:"userInfo,omitempty"`
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

func (n NameWithGVK) IsValid() bool {
	return n.Name != "" && n.Kind != ""
}

func (n NameWithGVK) String() string {
	return n.Kind + ":" + n.NamespacedName.String()
}

func (query NameWithGVK) Match(given NameWithGVK) bool {
	if !query.IsValid() || !given.IsValid() {
		return false
	}
	if query.Name != given.Name {
		return false
	}
	if query.Kind != given.Kind {
		return false
	}
	// Check namespace only if query has namespace
	if query.Namespace != "" && query.Namespace != given.Namespace {
		return false
	}
	// If group is empty, it is considered as a match
	if query.Group == "" {
		return true
	}
	if query.Group != given.Group {
		return false
	}
	// If version is empty, it is considered as a match
	if query.Version == "" {
		return true
	}
	if query.Version != given.Version {
		return false
	}
	return true
}

// NewNameWithGVKFromObj creates NameWithGVK from unstructured object.
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

// NewNameWithGVK creates NameWithGVK from GVK and NamespacedName.
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

// UserInfo is a struct to represent user information to populate request.userInfo.
type UserInfo struct {
	Name   string   `yaml:"name"`
	Groups []string `yaml:"groups"`
	Extra  map[string][]string
}

func NewK8sUserInfo(u UserInfo) user.DefaultInfo {
	return user.DefaultInfo{
		Name:   u.Name,
		Groups: u.Groups,
		Extra:  u.Extra,
	}
}
