package validator

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"
)

func TestValidatorBasic(t *testing.T) {
	v := &Validator{}
	if !v.Valid() {
		t.Errorf("expected empty validator to be valid")
	}

	v.AddFieldError("name", "cannot be blank")
	if v.Valid() {
		t.Errorf("expected validator with field error to be invalid")
	}

	v.AddNonFieldError("general failure")
	if v.Valid() {
		t.Errorf("expected validator with non-field error to be invalid")
	}

	if _, ok := v.FieldErrors["name"]; !ok {
		t.Errorf("expected field error for 'name'")
	}

	jsonData := v.JSON()
	var decoded Validator
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Errorf("failed to unmarshal JSON: %v", err)
	}
	if len(decoded.FieldErrors) == 0 || len(decoded.NonFieldErrors) == 0 {
		t.Errorf("expected JSON to contain errors")
	}
}

func TestCheckField(t *testing.T) {
	v := &Validator{}
	v.CheckField(false, "field1", "invalid")
	if len(v.FieldErrors["field1"]) == 0 {
		t.Errorf("expected error for field1")
	}
}

func TestStringValidators(t *testing.T) {
	if !NotBlank("hello") {
		t.Errorf("expected NotBlank to be true")
	}
	if NotBlank("   ") {
		t.Errorf("expected NotBlank to be false for spaces")
	}

	if !Blank("   ") {
		t.Errorf("expected Blank to be true for spaces")
	}
	if Blank("hi") {
		t.Errorf("expected Blank to be false")
	}

	if !MaxChars("abc", 3) || MaxChars("abcd", 3) {
		t.Errorf("MaxChars failed")
	}

	if !MinChars("abcd", 3) || MinChars("ab", 3) {
		t.Errorf("MinChars failed")
	}

	pattern := regexp.MustCompile(`^h.llo$`)
	if !Matches("hello", pattern) || Matches("world", pattern) {
		t.Errorf("Matches failed")
	}
}

func TestNumericValidators(t *testing.T) {
	if !IsNumber("123") || IsNumber("abc") {
		t.Errorf("IsNumber failed")
	}

	if !MinInt(5, 3) || MinInt(2, 3) {
		t.Errorf("MinInt failed")
	}

	if !MaxInt(3, 5) || MaxInt(6, 5) {
		t.Errorf("MaxInt failed")
	}

	if !MinFloat(3.5, 3.0) || MinFloat(2.9, 3.0) {
		t.Errorf("MinFloat failed")
	}

	if !MaxFloat(3.5, 4.0) || MaxFloat(5.1, 5.0) {
		t.Errorf("MaxFloat failed")
	}
}

func TestDurationValidators(t *testing.T) {
	if !MaxDuration(5*time.Second, 10*time.Second) || MaxDuration(11*time.Second, 10*time.Second) {
		t.Errorf("MaxDuration failed")
	}

	if !MinDuration(5*time.Second, 3*time.Second) || MinDuration(2*time.Second, 3*time.Second) {
		t.Errorf("MinDuration failed")
	}
}

func TestPermittedValue(t *testing.T) {
	if !PermittedValue("a", "a", "b", "c") {
		t.Errorf("expected value to be permitted")
	}
	if PermittedValue("z", "a", "b", "c") {
		t.Errorf("expected value not to be permitted")
	}

	if !PermittedValue(10, 5, 10, 15) {
		t.Errorf("expected int to be permitted")
	}
	if PermittedValue(20, 5, 10, 15) {
		t.Errorf("expected int not to be permitted")
	}
}
