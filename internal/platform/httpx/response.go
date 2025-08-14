package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Envelope map[string]any

func URLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func ReadIDParam(r *http.Request) (uuid.UUID, error) {
	idStr := chi.URLParam(r, "id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, errors.New("invalid id parameter")
	}

	return id, nil
}

func ReadJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf(
				"body contains badly-formed JSON (at character %d)",
				syntaxError.Offset,
			)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf(
					"body contains incorrect JSON type for filed %q",
					unmarshalTypeError.Field,
				)
			}
			return fmt.Errorf(
				"body contains incorrect JSON type (at character %d)",
				unmarshalTypeError.Offset,
			)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case errors.As(err, &maxBytesError):
			return fmt.Errorf(
				"body must not be larger than %d bytes", maxBytesError.Limit,
			)
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body myst only contain a single JSON value")
	}

	return nil
}

func WriteJSON(
	w http.ResponseWriter, status int, data Envelope, headers http.Header,
) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(js)

	return nil
}

func ErrorJSON(w http.ResponseWriter, status int, message any) {
	_ = WriteJSON(w, status, Envelope{"error": message}, nil)
}

func NotFound(w http.ResponseWriter, _ *http.Request) {
	ErrorJSON(w, http.StatusNotFound, "the requested resource could not be found")
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	ErrorJSON(w, http.StatusMethodNotAllowed, fmt.Sprintf(
		"the %s method is not supported for this resource", r.Method))
}

func BadRequest(w http.ResponseWriter, _ *http.Request, err error) {
	ErrorJSON(w, http.StatusBadRequest, err.Error())
}

func InternalError(w http.ResponseWriter, _ *http.Request, err error) {
	slog.Error("internal server error", "error", err)
	ErrorJSON(w, http.StatusInternalServerError,
		"the server encountered a problem and could not process your request")
}

func ValidationError(w http.ResponseWriter, _ *http.Request, errors map[string]string) {
	ErrorJSON(w, http.StatusUnprocessableEntity, errors)
}

func ServiceUnavailable(w http.ResponseWriter, _ *http.Request, err error) {
	slog.Error("service unavailable", "error", err)
	ErrorJSON(w, http.StatusServiceUnavailable,
		"the service is temporarily unavailable or unhealthy")
}

func Success(w http.ResponseWriter, status int, data any) {
	_ = WriteJSON(w, status, Envelope{"data": data}, nil)
}
