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

import "testing"

func TestRun(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{
			name: "ok",
			args: []string{
				"./testdata/vap-standard-resources.test/kaptest.yaml",
				"./testdata/vap-custom-resources.test/kaptest.yaml",
				"./testdata/vap-with-params.test/kaptest.yaml",
				"./testdata/vap-with-namespaces.test/kaptest.yaml",
				"./testdata/vap-with-userinfo.test/kaptest.yaml",
				"./testdata/map-standard-resources.test/kaptest.yaml",
				"./testdata/map-with-params.test/kaptest.yaml",
				"./testdata/map-with-namespaces.test/kaptest.yaml",
				"./testdata/map-with-userinfo.test/kaptest.yaml",
			},
			wantErr: nil,
		},
		{
			name:    "err: file not found",
			args:    []string{"./testdata/not-found.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name:    "err: unmarshal error",
			args:    []string{"./testdata/invalid-format.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name:    "err: invalid config",
			args:    []string{"./testdata/invalid-config.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name:    "err: policy file not found",
			args:    []string{"./testdata/invalid-policy-file-not-found.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name:    "err: policy not exist",
			args:    []string{"./testdata/vap-standard-resources.test/invalid-no-policy.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name:    "err: object not exist",
			args:    []string{"./testdata/vap-standard-resources.test/invalid-no-obj.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name:    "err: object not exist (custom resource)",
			args:    []string{"./testdata/vap-custom-resources.test/invalid-no-obj.yaml"},
			wantErr: ErrTestFail,
		},
		{
			name: "err: params not exist",
			args: []string{
				"./testdata/vap-with-params.test/invalid-no-params.yaml",
				"./testdata/map-with-params.test/invalid-no-params.yaml",
			},
			wantErr: ErrTestFail,
		},
		{
			name: "err: namespace not exist",
			args: []string{
				"./testdata/vap-with-namespaces.test/invalid-no-namespace.yaml",
				"./testdata/map-with-namespaces.test/invalid-no-namespace.yaml",
			},
			wantErr: ErrTestFail,
		},
	}
	for _, tt := range tests {
		cfg := CmdConfig{Verbose: true}
		t.Run(tt.name, func(t *testing.T) {
			got := Run(cfg, tt.args)
			if got != tt.wantErr {
				t.Errorf("Run() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}
