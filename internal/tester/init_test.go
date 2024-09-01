package tester

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/utils/ptr"
)

func TestRunInit(t *testing.T) {
	t.Parallel()
	y := printers.YAMLPrinter{}
	manifestFile := "policy.yaml"
	testDir := "policy.test"

	tests := []struct {
		name  string
		setup func(tmpDir string, manifestFile io.Writer)
	}{
		{
			name: "ok",
			setup: func(tmpDir string, manifestFile io.Writer) {
				// nop
			},
		},
		{
			name: "ok: test dir already exists",
			setup: func(tmpDir string, manifestFile io.Writer) {
				mustNil(t, os.Mkdir(filepath.Join(tmpDir, testDir), 0755))
			},
		},
		{
			name: "ok: other resources are included in the policy file",
			setup: func(tmpDir string, manifestFile io.Writer) {
				mustNil(t, y.PrintObj(dummyDeployment(), manifestFile))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			manifestPath := filepath.Join(dir, manifestFile)
			f, _ := os.Create(manifestPath)
			mustNil(t, y.PrintObj(sampleValidatingAdmissionPolicy(), f))
			tt.setup(dir, f)

			if err := RunInit(CmdConfig{Verbose: true}, manifestPath); err != nil {
				t.Errorf("RunInit() = %v, want nil", err)
				return
			}

			// Check the test directory
			info, err := os.Stat(filepath.Join(dir, testDir))
			if err != nil {
				t.Errorf("root manifest file is not generated: %v", err)
				return
			}
			if !info.IsDir() {
				t.Errorf("test directory is not generated")
				return
			}

			// Check the root manifest file
			buf, err := os.ReadFile(filepath.Join(dir, testDir, rootManifestName))
			if err != nil {
				t.Errorf("root manifest file is not generated: %v", err)
				return
			}
			if string(buf) != string(wantRootManifest()) {
				t.Errorf("root manifest content is not as expected: %s", buf)
			}

			// Check the resource manifest file
			_, err = os.Stat(filepath.Join(dir, testDir, resourceManifestName))
			if err != nil {
				t.Errorf("resource manifest file is not generated: %v", err)
			}
		})
	}

	t.Run("err: file not found", func(t *testing.T) {
		if err := RunInit(CmdConfig{Verbose: true}, "./not-found.yaml"); err == nil {
			t.Error("RunInit() = nil, want error")
		}
	})

	t.Run("err: root manifest file already exists", func(t *testing.T) {
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, manifestFile)
		f, _ := os.Create(manifestPath)
		mustNil(t, y.PrintObj(sampleValidatingAdmissionPolicy(), f))
		mustNil(t, os.Mkdir(filepath.Join(dir, testDir), 0755))
		mustNil(t, os.WriteFile(filepath.Join(dir, testDir, rootManifestName), []byte{}, 0644))

		if err := RunInit(CmdConfig{Verbose: true}, manifestPath); err == nil {
			t.Error("RunInit() = nil, want error")
		}
	})
}

func sampleValidatingAdmissionPolicy() *v1.ValidatingAdmissionPolicy {
	vap := &v1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-policy",
		},
		Spec: v1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(v1.Fail),
			MatchConstraints: &v1.MatchResources{
				ResourceRules: []v1.NamedRuleWithOperations{
					{
						RuleWithOperations: v1.RuleWithOperations{
							Rule: v1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
							Operations: []v1.OperationType{"CREATE", "UPDATE"},
						},
					},
				},
			},
			Validations: []v1.Validation{
				{
					Expression: "object.spec.replicas <= 5",
					Message:    "object.spec.replicas should less or equal to 5",
				},
			},
		},
	}
	vap.GetObjectKind().SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy"))
	return vap
}

func dummyDeployment() *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dummy",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(int32(3)),
		},
	}
	d.GetObjectKind().SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	return d
}

func wantRootManifest() []byte {
	m := TestManifests{
		ValidatingAdmissionPolicies: []string{"../policy.yaml"},
		Resources:                   []string{"resources.yaml"},
		TestSuites: []TestsForSinglePolicy{
			{
				Policy: "sample-policy",
				Tests: []TestCase{
					{
						Object: NameWithGVK{
							GVK:            GVK{Kind: "Pod"},
							NamespacedName: NamespacedName{Name: "ok"},
						},
						Expect: Admit,
					},
					{
						Object: NameWithGVK{
							GVK:            GVK{Kind: "Pod"},
							NamespacedName: NamespacedName{Name: "bad"},
						},
						Expect: Deny,
					},
				},
			},
		},
	}
	buf, _ := yaml.Marshal(m)
	return buf
}

func mustNil(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		panic(err)
	}
}
