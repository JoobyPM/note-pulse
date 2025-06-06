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
func RegisterPasswordValidator(v *validator.Validate) error {
	// Try to register
	err := v.RegisterValidation("password", cryptoPasswordRule)
	if err != nil {
		return err
	}
	return nil
}
