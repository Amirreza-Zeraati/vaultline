// Package response standardizes JSON output so every endpoint returns the same
// shape: {"data": ...} on success, {"error": {...}} on failure.
//
// Handlers should call Fail(c, err) and let it derive the status code from the
// error, rather than choosing a status themselves.
package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
)

// Envelope is the success shape.
type Envelope struct {
	Data any `json:"data"`
}

// ErrorBody is the failure shape.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail is the error payload. Code is stable and machine-readable;
// Message is human-readable; Fields is populated for validation failures.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// JSON writes a success payload.
func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, Envelope{Data: data})
}

// Fail writes an error response derived from err. Server-side faults (5xx) are
// logged with their wrapped cause; the client only ever sees the safe message.
func Fail(c *gin.Context, err error) {
	appErr, body := prepare(c, err)
	c.JSON(appErr.Status, body)
}

// AbortFail is Fail for use inside middleware: it also stops the chain so no
// downstream handler runs.
func AbortFail(c *gin.Context, err error) {
	appErr, body := prepare(c, err)
	c.AbortWithStatusJSON(appErr.Status, body)
}

// prepare converts err to an *apperr.Error, logs it if it's a server fault, and
// builds the wire body.
func prepare(c *gin.Context, err error) (*apperr.Error, ErrorBody) {
	appErr := apperr.From(err)

	if appErr.IsServer() {
		// Log the full chain, including the wrapped cause, which is never sent
		// to the client.
		slog.Error("request failed",
			"code", string(appErr.Code),
			"status", appErr.Status,
			"path", c.Request.URL.Path,
			"method", c.Request.Method,
			"err", appErr.Error(),
		)
	}

	return appErr, ErrorBody{Error: ErrorDetail{
		Code:    string(appErr.Code),
		Message: appErr.Message,
		Fields:  appErr.Fields,
	}}
}

// BindError converts a Gin binding error into an *apperr.Error: a validator
// failure becomes a 422 with per-field messages, anything else a 400.
func BindError(err error) *apperr.Error {
	// A body that blew past the BodyLimit middleware surfaces here; report it
	// as 413 rather than a confusing generic 400.
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return apperr.PayloadTooLarge("request body too large")
	}

	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := make(map[string]string, len(ve))
		for _, fe := range ve {
			fields[fe.Field()] = messageFor(fe)
		}
		return apperr.Validation("validation failed").WithFields(fields)
	}
	return apperr.InvalidInput("invalid request body").Wrap(err)
}

func messageFor(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email"
	case "min":
		return "is too short (min " + fe.Param() + ")"
	case "max":
		return "is too long (max " + fe.Param() + ")"
	default:
		return "is invalid"
	}
}
