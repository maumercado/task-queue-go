package logger

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	// Test that Init doesn't panic
	Init("info", false)
	assert.NotNil(t, Get())

	Init("debug", true)
	assert.NotNil(t, Get())
}

func TestInit_LogLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"invalid", zerolog.InfoLevel}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			Init(tt.level, false)
			assert.Equal(t, tt.expected, zerolog.GlobalLevel())
		})
	}
}

func TestGet(t *testing.T) {
	Init("info", false)
	logger := Get()
	assert.NotNil(t, logger)
}

func TestWithComponent(t *testing.T) {
	Init("info", false)

	var buf bytes.Buffer
	log = zerolog.New(&buf)

	componentLogger := WithComponent("api")
	componentLogger.Info().Msg("test message")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "api", logEntry["component"])
	assert.Equal(t, "test message", logEntry["message"])
}

func TestWithWorker(t *testing.T) {
	Init("info", false)

	var buf bytes.Buffer
	log = zerolog.New(&buf)

	workerLogger := WithWorker("worker-123")
	workerLogger.Info().Msg("worker message")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "worker-123", logEntry["worker_id"])
}

func TestWithTask(t *testing.T) {
	Init("info", false)

	var buf bytes.Buffer
	log = zerolog.New(&buf)

	taskLogger := WithTask("task-456")
	taskLogger.Info().Msg("task message")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "task-456", logEntry["task_id"])
}

func TestLogLevelMethods(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	// Test Debug
	Debug().Msg("debug message")
	assert.Contains(t, buf.String(), "debug message")
	buf.Reset()

	// Test Info
	Info().Msg("info message")
	assert.Contains(t, buf.String(), "info message")
	buf.Reset()

	// Test Warn
	Warn().Msg("warn message")
	assert.Contains(t, buf.String(), "warn message")
	buf.Reset()

	// Test Error
	Error().Msg("error message")
	assert.Contains(t, buf.String(), "error message")
	buf.Reset()
}

func TestLogLevels_Filtered(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	// Debug and Info should be filtered
	Debug().Msg("debug message")
	assert.Empty(t, buf.String())

	Info().Msg("info message")
	assert.Empty(t, buf.String())

	// Warn and Error should pass through
	Warn().Msg("warn message")
	assert.Contains(t, buf.String(), "warn message")
	buf.Reset()

	Error().Msg("error message")
	assert.Contains(t, buf.String(), "error message")
}

func TestLogWithFields(t *testing.T) {
	var buf bytes.Buffer
	log = zerolog.New(&buf)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	Info().
		Str("key1", "value1").
		Int("key2", 42).
		Bool("key3", true).
		Msg("structured log")

	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)

	assert.Equal(t, "value1", logEntry["key1"])
	assert.Equal(t, float64(42), logEntry["key2"]) // JSON numbers are float64
	assert.Equal(t, true, logEntry["key3"])
	assert.Equal(t, "structured log", logEntry["message"])
}
