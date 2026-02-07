package validator

import (
	"fmt"
)

// ValidateRequest validates a request that implements the Validatable interface
// and returns an error if validation fails
func ValidateRequest(req Validatable) error {
	v := &Validator{}
	req.Validate(v)
	if !v.Valid() {
		return fmt.Errorf("validation failed: %s", string(v.JSON()))
	}
	return nil
}

// ValidateRequestWithValidator validates a request using a provided validator
// This allows for custom validation logic or reusing a validator instance
func ValidateRequestWithValidator(req Validatable, v *Validator) error {
	req.Validate(v)
	if !v.Valid() {
		return fmt.Errorf("validation failed: %s", string(v.JSON()))
	}
	return nil
}
