package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHandler_respondJSON(t *testing.T) {
	h := &AdminHandler{}

	w := httptest.NewRecorder()
	data := map[string]string{"status": "ok"}

	h.respondJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestAdminHandler_respondError(t *testing.T) {
	h := &AdminHandler{}

	w := httptest.NewRecorder()
	h.respondError(w, http.StatusNotFound, "worker not found")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Not Found", response["error"])
	assert.Equal(t, "worker not found", response["message"])
}

func TestAdminHandler_GetWorker_MissingID(t *testing.T) {
	h := &AdminHandler{}

	req := httptest.NewRequest(http.MethodGet, "/admin/workers/", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workerID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.GetWorker(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "worker ID is required", response["message"])
}

func TestAdminHandler_RetryTask_MissingID(t *testing.T) {
	h := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/tasks//retry", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("taskID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.RetryTask(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "task ID is required", response["message"])
}

func TestAdminHandler_PauseWorker_MissingID(t *testing.T) {
	h := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/workers//pause", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workerID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.PauseWorker(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_ResumeWorker_MissingID(t *testing.T) {
	h := &AdminHandler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/workers//resume", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workerID", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.ResumeWorker(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_PurgeQueue_MissingPriority(t *testing.T) {
	h := &AdminHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/admin/queues/", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("priority", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.PurgeQueue(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminHandler_PurgeQueue_InvalidPriority(t *testing.T) {
	h := &AdminHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/admin/queues/invalid", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("priority", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.PurgeQueue(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response["message"], "invalid priority")
}

func TestRetryDLQRequest_Struct(t *testing.T) {
	req := RetryDLQRequest{
		TaskID:    "task-123",
		RetryAll:  false,
		MessageID: "msg-456",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded RetryDLQRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, req.TaskID, decoded.TaskID)
	assert.Equal(t, req.RetryAll, decoded.RetryAll)
	assert.Equal(t, req.MessageID, decoded.MessageID)
}

func TestRetryDLQRequest_RetryAll(t *testing.T) {
	req := RetryDLQRequest{
		RetryAll: true,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded RetryDLQRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.RetryAll)
	assert.Empty(t, decoded.TaskID)
}
