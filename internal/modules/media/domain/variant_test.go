package media

import "testing"

func TestVariantLifecycle(t *testing.T) {
	width := int32(320)
	height := int32(240)

	variant := Variant{
		Name:   VariantNameCard4x3,
		Status: VariantStatusPending,
	}

	if err := variant.StartProcessing(); err != nil {
		t.Fatalf("StartProcessing() error = %v", err)
	}
	if err := variant.MarkReady(&width, &height); err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}
	if variant.Status != VariantStatusReady {
		t.Fatalf("expected ready, got %s", variant.Status)
	}
}

func TestVariantRejectsInvalidTransition(t *testing.T) {
	variant := Variant{Status: VariantStatusPending}
	if err := variant.MarkReady(nil, nil); err == nil {
		t.Fatal("expected invalid transition error")
	}
}
