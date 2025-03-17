package quant

import (
	"testing"
)

func TestRequestBscMev(t *testing.T) {
	builder := NewBuilder()
	validators, err := builder.RequestBscMevValidators()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedValidators := []string{"0x18a...a09", "0xe91..c10"}
	if !equal(validators, expectedValidators) {
		t.Fatalf("expected %v, got %v", expectedValidators, validators)
	}
}

func TestRequestBlxrSubmitBundle(t *testing.T) {

	builder := NewBuilder()
	err := builder.RequestBlxrSubmitBundle()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRequestBlxrSimulateBundle(t *testing.T) {

	builder := NewBuilder()
	err := builder.RequestBlxrSimulateBundle()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
