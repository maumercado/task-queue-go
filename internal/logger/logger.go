package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var log zerolog.Logger

func Init(level string, pretty bool) {
	// Parse log level
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(lvl)

	var output io.Writer = os.Stdout
	if pretty {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}
	}

	log = zerolog.New(output).
		With().
		Timestamp().
		Caller().
		Logger()
}

func Get() *zerolog.Logger {
	return &log
}

func WithComponent(component string) zerolog.Logger {
	return log.With().Str("component", component).Logger()
}

func WithWorker(workerID string) zerolog.Logger {
	return log.With().Str("worker_id", workerID).Logger()
}

func WithTask(taskID string) zerolog.Logger {
	return log.With().Str("task_id", taskID).Logger()
}

// Convenience methods
func Debug() *zerolog.Event {
	return log.Debug()
}

func Info() *zerolog.Event {
	return log.Info()
}

func Warn() *zerolog.Event {
	return log.Warn()
}

func Error() *zerolog.Event {
	return log.Error()
}

func Fatal() *zerolog.Event {
	return log.Fatal()
}
