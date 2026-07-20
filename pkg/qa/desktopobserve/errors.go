package desktopobserve

import "errors"

var (
	ErrDuplicateKey            = errors.New("desktop observation envelope contains a duplicate key")
	ErrEnvelopeTooLarge        = errors.New("desktop observation envelope exceeds the byte limit")
	ErrFailureSignalInvalid    = errors.New("desktop observation failure signal is invalid")
	ErrInvalidStatus           = errors.New("desktop observation result status is invalid")
	ErrMalformedEnvelope       = errors.New("desktop observation envelope is malformed")
	ErrMissingField            = errors.New("desktop observation envelope is missing a required field")
	ErrProtocolMismatch        = errors.New("desktop observation protocol version mismatch")
	ErrRequestIDMismatch       = errors.New("desktop observation request ID mismatch")
	ErrRuntimeProviderInvalid  = errors.New("desktop observation runtime provider is invalid")
	ErrRuntimeProviderRequired = errors.New("desktop observation runtime provider is required")
	ErrScopeMismatch           = errors.New("desktop observation scope binding mismatch")
	ErrUnknownField            = errors.New("desktop observation envelope contains an unknown field")
	ErrUnsupportedOperation    = errors.New("desktop observation operation is not read-only")
	ErrRawOnlyEvidence         = errors.New("desktop observation has only quarantined raw evidence")
	ErrRedactionFailed         = errors.New("desktop observation redaction failed")
)

type reasonError struct {
	code ReasonCode
}

func (err reasonError) Error() string {
	return string(err.code)
}
