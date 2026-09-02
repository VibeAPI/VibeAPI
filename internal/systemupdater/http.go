package systemupdater

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	contract "github.com/QuantumNous/new-api/pkg/systemupdate"
)

type API struct {
	engine *Engine
	token  string
}

func NewHandler(engine *Engine, token string) http.Handler {
	api := &API{engine: engine, token: token}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/capabilities", api.capabilities)
	mux.HandleFunc("GET /v1/releases", api.releases)
	mux.HandleFunc("POST /v1/preflight", api.preflight)
	mux.HandleFunc("POST /v1/operations", api.startOperation)
	mux.HandleFunc("GET /v1/operations", api.listOperations)
	mux.HandleFunc("GET /v1/operations/{id}", api.getOperation)
	return api.authenticate(mux)
}

func (api *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		provided := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if api.token == "" || len(provided) != len(api.token) || subtle.ConstantTimeCompare([]byte(provided), []byte(api.token)) != 1 {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			writeError(writer, http.StatusUnauthorized, "unauthorized", "valid updater bearer token required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (api *API) capabilities(writer http.ResponseWriter, request *http.Request) {
	capability, err := api.engine.Capabilities(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "capability_failed", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, capability)
}

func (api *API) releases(writer http.ResponseWriter, request *http.Request) {
	releases, err := api.engine.Releases(request.Context())
	if err != nil {
		writeError(writer, http.StatusServiceUnavailable, "catalog_unavailable", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, releases)
}

func (api *API) preflight(writer http.ResponseWriter, request *http.Request) {
	var input contract.PreflightRequest
	if !decodeRequest(writer, request, &input) {
		return
	}
	result, err := api.engine.Preflight(request.Context(), input)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "preflight_failed", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, result)
}

func (api *API) startOperation(writer http.ResponseWriter, request *http.Request) {
	var input contract.StartOperationRequest
	if !decodeRequest(writer, request, &input) {
		return
	}
	operation, err := api.engine.Start(request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrUpdateLocked) {
			writeError(writer, http.StatusConflict, "operation_locked", err.Error())
			return
		}
		writeError(writer, http.StatusBadRequest, "operation_rejected", err.Error())
		return
	}
	writeJSON(writer, http.StatusAccepted, operation)
}

func (api *API) listOperations(writer http.ResponseWriter, _ *http.Request) {
	operations, err := api.engine.Operations()
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "journal_failed", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"operations": operations})
}

func (api *API) getOperation(writer http.ResponseWriter, request *http.Request) {
	operation, err := api.engine.Operation(request.PathValue("id"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(writer, http.StatusNotFound, "operation_not_found", "operation not found")
			return
		}
		writeError(writer, http.StatusInternalServerError, "journal_failed", err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, operation)
}

func decodeRequest(writer http.ResponseWriter, request *http.Request, output any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	if err := common.DecodeJson(request.Body, output); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	data, err := common.Marshal(value)
	if err != nil {
		http.Error(writer, "json encoding failed", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_, _ = writer.Write(append(data, '\n'))
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
