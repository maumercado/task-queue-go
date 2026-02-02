package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maumercado/task-queue-go/internal/logger"
	"github.com/maumercado/task-queue-go/internal/task"
)

func init() {
	logger.Init("error", false)
}

func TestTaskHandler_respondJSON(t *testing.T) {
	h := &TaskHandler{}

	w := httptest.NewRecorder()
	data := map[string]string{"message": "hello"}

	h.respondJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "hello", response["message"])
}

func TestTaskHandler_respondError(t *testing.T) {
	h := &TaskHandler{}

	w := httptest.NewRecorder()
	h.respondError(w, http.StatusBadRequest, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Bad Request", response.Error)
	assert.Equal(t, "invalid input", response.Message)
}

func TestTaskHandler_Create_InvalidJSON(t *testing.T) {
	h := &TaskHandler{}

	body := bytes.NewBufferString("invalid json")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid request body", response.Message)
}

func TestTaskHandler_Create_MissingType(t *testing.T) {
	h := &TaskHandler{}

	reqBody := task.CreateTaskRequest{
		Type:    "", // Empty type
		Payload: map[string]interface{}{"key": "value"},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "task type is required", response.Message)
}

func TestTaskHandler_Get_MissingID(t *testing.T) {
	h := &TaskHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/", nil)
	w := httptest.NewRecorder()

	// Create a chi context with empty taskID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("taskID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.Get(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTaskHandler_Cancel_MissingID(t *testing.T) {
	h := &TaskHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("taskID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.Cancel(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestErrorResponse_Struct(t *testing.T) {
	resp := ErrorResponse{
		Error:   "Not Found",
		Message: "Task not found",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded ErrorResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, resp.Error, decoded.Error)
	assert.Equal(t, resp.Message, decoded.Message)
}

func TestListResponse_Struct(t *testing.T) {
	resp := ListResponse{
		Tasks: []*task.TaskResponse{
			{
				ID:       "task-1",
				Type:     "email",
				Priority: "high",
				State:    "pending",
			},
		},
		TotalCount: 1,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded ListResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 1, decoded.TotalCount)
	assert.Len(t, decoded.Tasks, 1)
	assert.Equal(t, "task-1", decoded.Tasks[0].ID)
}
