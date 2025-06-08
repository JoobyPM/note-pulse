package util

import (
	"context"

	"note-pulse/internal/utils/crypto"

	"github.com/go-playground/validator/v10"
)

// ValidateCtx executes v.StructCtx with an error bucket wired-in.
// Returns crypto.ErrPasswordWeak verbatim when that rule fails.
func ValidateCtx(ctx context.Context, v *validator.Validate, req any) error {
	var pwdErr error
	vctx := context.WithValue(ctx, crypto.PasswordErrKey, &pwdErr)

	if err := v.StructCtx(vctx, req); err != nil || pwdErr != nil {
		if pwdErr != nil {
			return pwdErr
		}
		return err
	}
	return nil
}
