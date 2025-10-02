package util

import (
	"testing"
)

func TestGetSupportedKubernetesVersion(t *testing.T) {
	if GetSupportedKubernetesVersion() != "1.32.1" {
		t.Fatalf("unexpected version")
	}
}
