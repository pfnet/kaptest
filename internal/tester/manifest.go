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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
)

// TestManifests is a struct to represent the whole test manifest file.
type TestManifests struct {
	Policies        []string                  `yaml:"policies,omitempty"`
	Resources       []string                  `yaml:"resources,omitempty"`
	SchemaLocations []string                  `yaml:"schemaLocations,omitempty"` // used for resource manifest validation
	VapTestSuites   []TestsForSingleVapPolicy `yaml:"vapTestSuites,omitempty"`
	MapTestSuites   []TestsForSingleMapPolicy `yaml:"mapTestSuites,omitempty"`
}

func (t TestManifests) IsValid() (bool, string) {
	if len(t.Policies) == 0 {
		return false, "at least one policies is required"
	}
	if len(t.Resources) == 0 {
		return false, "at least one resources is required"
	}
	if len(t.VapTestSuites) == 0 && len(t.MapTestSuites) == 0 {
		return false, "at least one vapTestSuites or mapTestSuites is required"
	}
	return true, ""
}

// TestsForSingleVapPolicy is a struct to aggregate multiple test cases for a single policy.
type TestsForSingleVapPolicy struct {
	Policy string        `yaml:"policy"`
	Tests  []VAPTestCase `yaml:"tests"`
}

type PolicyDecisionExpect string

const (
	Admit  PolicyDecisionExpect = "admit"
	Deny   PolicyDecisionExpect = "deny"
	Error  PolicyDecisionExpect = "error"
	Skip   PolicyDecisionExpect = "skip"
	Mutate PolicyDecisionExpect = "mutate"
)

type TestCase interface {
	GetExpect() PolicyDecisionExpect
	SummaryLine(pass bool, policy string, result string) string
}

// TestCase is a struct to represent a single test case.
type VAPTestCase struct {
	Object    NameWithGVK          `yaml:"object,omitempty"`
	OldObject NameWithGVK          `yaml:"oldObject,omitempty"`
	Param     NamespacedName       `yaml:"param,omitempty"`
	Expect    PolicyDecisionExpect `yaml:"expect,omitempty"`
	UserInfo  UserInfo             `yaml:"userInfo,omitempty"`
	// TODO: Support message test
	// Message   string                              `yaml:"message"`
}

var _ TestCase = &VAPTestCase{}

func (tc VAPTestCase) GetExpect() PolicyDecisionExpect {
	return tc.Expect
}

func (tc VAPTestCase) SummaryLine(pass bool, policy string, result string) string {
	summary := "[VAP]"
	if pass {
		summary += " PASS"
	} else {
		summary += " FAIL"
	}

	summary += fmt.Sprintf(": %s", policy)
	if tc.Object.IsValid() && tc.OldObject.IsValid() { //nolint:gocritic
		summary += fmt.Sprintf(" - (UPDATE) %s -> %s", tc.OldObject.String(), tc.Object.NamespacedName.String())
	} else if tc.Object.IsValid() {
		summary += fmt.Sprintf(" - (CREATE) %s", tc.Object.String())
	} else if tc.OldObject.IsValid() {
		summary += fmt.Sprintf(" - (DELETE) %s", tc.OldObject.String())
	}
	if tc.Param.IsValid() {
		summary += fmt.Sprintf(" (Param: %s)", tc.Param.String())
	}
	summary += fmt.Sprintf(" - %s ==> %s", strings.ToUpper(string(tc.Expect)), strings.ToUpper(result))
	return summary
}

type TestsForSingleMapPolicy struct {
	Policy string        `yaml:"policy"`
	Tests  []MAPTestCase `yaml:"tests"`
}

type MAPTestCase struct {
	Object               NameWithGVK          `yaml:"object,omitempty"`
	OldObject            NameWithGVK          `yaml:"oldObject,omitempty"`
	Param                NamespacedName       `yaml:"param,omitempty"`
	Expect               PolicyDecisionExpect `yaml:"expect"`
	ExpectObject         NameWithGVK          `yaml:"expectObject,omitempty"`
	UserInfo             UserInfo             `yaml:"userInfo,omitempty"`
	DisableNameOverwrite bool                 `yaml:"disableNameOverwrite,omitempty"`
}

var _ TestCase = &MAPTestCase{}

func (tc MAPTestCase) GetExpect() PolicyDecisionExpect {
	return tc.Expect
}

func (tc MAPTestCase) SummaryLine(pass bool, policy string, result string) string {
	summary := "[MAP]"
	if pass {
		summary += " PASS"
	} else {
		summary += " FAIL"
	}

	summary += fmt.Sprintf(": %s", policy)
	if tc.Object.IsValid() && tc.OldObject.IsValid() { //nolint:gocritic
		summary += fmt.Sprintf(" - (UPDATE) %s -> %s", tc.OldObject.String(), tc.Object.NamespacedName.String())
	} else if tc.Object.IsValid() {
		summary += fmt.Sprintf(" - (CREATE) %s", tc.Object.String())
	} else if tc.OldObject.IsValid() {
		summary += fmt.Sprintf(" - (DELETE) %s", tc.OldObject.String())
	}
	summary += fmt.Sprintf(" - %s ==> %s", strings.ToUpper(string(tc.Expect)), strings.ToUpper(result))
	return summary
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
