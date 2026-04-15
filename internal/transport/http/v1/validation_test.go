package v1

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/go-playground/validator/v10"

	"cixing/internal/transport/http/server/response"
)

type validationTestPayload struct {
	Email  string                   `json:"email" validate:"required,email"`
	Nested validationTestNestedBody `json:"nested"`
}

type validationTestNestedBody struct {
	Count int `json:"count" validate:"min=1"`
}

func TestBindFieldErrors_FromValidatorErrors(t *testing.T) {
	validate := validator.New()
	payload := validationTestPayload{
		Nested: validationTestNestedBody{Count: 0},
	}

	err := validate.Struct(payload)
	got := bindFieldErrors(payload, err)
	want := []response.FieldError{
		{Field: "email", Rule: "required", Reason: "is required"},
		{Field: "nested.count", Rule: "min", Reason: "must be at least 1"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bindFieldErrors() = %#v, want %#v", got, want)
	}
}

func TestBindFieldErrors_FromUnmarshalTypeError(t *testing.T) {
	var payload validationTestPayload
	err := json.Unmarshal([]byte(`{"nested":{"count":"oops"}}`), &payload)
	if err == nil {
		t.Fatal("json.Unmarshal() error = nil, want type error")
	}

	got := bindFieldErrors(&payload, err)
	want := []response.FieldError{
		{Field: "nested.count", Rule: "type", Reason: "must be an integer"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bindFieldErrors() = %#v, want %#v", got, want)
	}
}
