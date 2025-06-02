package crypto

import (
	"github.com/go-playground/validator/v10"
)

// cryptoPasswordRule validates password strength for the validator package
func cryptoPasswordRule(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return IsStrong(password)
}

// RegisterPasswordValidator registers the "password" validation tag with the validator
// Safely handles duplicate registration by checking if already registered
func RegisterPasswordValidator(v *validator.Validate) error {
	// Try to register, if it fails due to duplicate, that's fine
	err := v.RegisterValidation("password", cryptoPasswordRule)
	if err != nil && err.Error() == "validator: tag 'password' already exists" {
		return nil // Already registered, not an error
	}
	return err
}
