package v1

import (
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"

	"cixing/internal/transport/http/server/response"
)

func bindFieldErrors(dst any, err error) []response.FieldError {
	if err == nil {
		return nil
	}

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		out := make([]response.FieldError, 0, len(validationErrs))
		for _, fieldErr := range validationErrs {
			field := jsonPathFromStructPath(dst, fieldErr.StructNamespace())
			if field == "" {
				field = toSnakeCase(fieldErr.Field())
			}
			out = append(out, response.FieldError{
				Field:  field,
				Rule:   validationRule(fieldErr),
				Reason: validationReason(fieldErr),
			})
		}
		if len(out) > 0 {
			return out
		}
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := jsonPathFromStructPath(dst, typeErr.Field)
		if field == "" {
			field = "body"
		}
		return []response.FieldError{{
			Field:  field,
			Rule:   "type",
			Reason: expectedTypeReason(typeErr.Type),
		}}
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return []response.FieldError{{
			Field:  "body",
			Rule:   "json",
			Reason: "must be valid JSON",
		}}
	}

	if errors.Is(err, io.EOF) {
		return []response.FieldError{{
			Field:  "body",
			Rule:   "required",
			Reason: "is required",
		}}
	}

	return []response.FieldError{{
		Field:  "body",
		Rule:   "invalid",
		Reason: err.Error(),
	}}
}

func jsonPathFromStructPath(root any, structPath string) string {
	structPath = strings.TrimSpace(structPath)
	if structPath == "" {
		return ""
	}

	current := indirectType(reflect.TypeOf(root))
	if current == nil {
		return ""
	}

	parts := strings.Split(structPath, ".")
	if len(parts) > 0 && parts[0] == current.Name() {
		parts = parts[1:]
	}

	jsonParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == nil {
			jsonParts = append(jsonParts, toSnakeCase(part))
			continue
		}

		current = indirectType(current)
		if current == nil || current.Kind() != reflect.Struct {
			jsonParts = append(jsonParts, toSnakeCase(part))
			current = nil
			continue
		}

		field, ok := current.FieldByName(part)
		if !ok {
			jsonParts = append(jsonParts, toSnakeCase(part))
			current = nil
			continue
		}

		jsonParts = append(jsonParts, jsonFieldName(field))
		current = field.Type
	}

	return strings.Join(jsonParts, ".")
}

func indirectType(t reflect.Type) reflect.Type {
	for t != nil {
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			t = t.Elem()
		default:
			return t
		}
	}
	return nil
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag != "" {
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			return name
		}
	}
	return toSnakeCase(field.Name)
}

func toSnakeCase(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func validationReason(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "uuid", "uuid4":
		return "must be a valid UUID"
	case "min":
		switch err.Kind() {
		case reflect.String:
			return "must be at least " + err.Param() + " characters"
		case reflect.Slice, reflect.Array:
			return "must contain at least " + err.Param() + " items"
		default:
			return "must be at least " + err.Param()
		}
	case "max":
		switch err.Kind() {
		case reflect.String:
			return "must be at most " + err.Param() + " characters"
		case reflect.Slice, reflect.Array:
			return "must contain at most " + err.Param() + " items"
		default:
			return "must be at most " + err.Param()
		}
	case "oneof":
		return "must be one of " + strings.ReplaceAll(err.Param(), " ", ", ")
	default:
		if param := strings.TrimSpace(err.Param()); param != "" {
			return "failed validation: " + err.Tag() + "=" + param
		}
		return "failed validation: " + err.Tag()
	}
}

func validationRule(err validator.FieldError) string {
	tag := strings.TrimSpace(err.Tag())
	if tag == "" {
		return "invalid"
	}
	return tag
}

func expectedTypeReason(t reflect.Type) string {
	if t == nil {
		return "has an invalid type"
	}

	switch t.Kind() {
	case reflect.Bool:
		return "must be a boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "must be an integer"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "must be an unsigned integer"
	case reflect.Float32, reflect.Float64:
		return "must be a number"
	case reflect.String:
		return "must be a string"
	case reflect.Slice, reflect.Array:
		return "must be an array"
	case reflect.Struct, reflect.Map:
		return "must be an object"
	default:
		return "has an invalid type"
	}
}
