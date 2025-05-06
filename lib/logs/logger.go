package logs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger       zerolog.Logger
	bufferWriter *BufferWriter
)

const defaultBufSize = 64 * 1024 // 64KB

type BufferWriter struct {
	mu  sync.Mutex
	buf *bytes.Buffer
	cap int
}

func NewBufferWriter(capacity int) *BufferWriter {
	if capacity <= 0 {
		capacity = defaultBufSize
	}
	return &BufferWriter{
		buf: bytes.NewBuffer(make([]byte, 0, capacity)),
		cap: capacity,
	}
}

func (w *BufferWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buf.Len()+len(p) > w.cap {
		drop := w.buf.Len() + len(p) - w.cap
		data := w.buf.Bytes()
		w.buf.Reset()
		w.buf.Write(data[drop:])
	}
	w.buf.Write(p)
	return len(p), nil
}

func (w *BufferWriter) GetAndClear() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.buf.String()
	w.buf.Reset()
	return s
}

// EnableInMemoryBuffer activates an in-memory circular buffer of given capacity
func EnableInMemoryBuffer(capacity int) {
	if bufferWriter == nil {
		bufferWriter = NewBufferWriter(capacity)
	}
}

// GetBufferedLogs returns and clears the in-memory buffer
func GetBufferedLogs() string {
	if bufferWriter == nil {
		return ""
	}
	return bufferWriter.GetAndClear()
}

// Init initializes the global logger.
// logType:    "stdout"|"file"|"both"|"off"
// logLevel:   "trace"|"debug"|"info"|"warn"|"error"|"fatal"|"panic"|"off"
// logPath:    file path (required for file/both)
// maxSize:    max size per file in MB
// maxBackups: max number of backups
// maxAge:     max age in days
// compress:   whether to compress old logs
// color:      whether to enable ANSI color codes in console output
func Init(
	logType, logLevel, logPath string,
	maxSize, maxBackups, maxAge int,
	compress bool,
	color bool,
) {
	lvlKey := strings.ToLower(logLevel)
	var lvl zerolog.Level
	switch lvlKey {
	case "0", "off", "disabled":
		lvl = zerolog.Disabled
	case "1", "panic", "emergency":
		lvl = zerolog.PanicLevel
	case "2", "fatal", "critical":
		lvl = zerolog.FatalLevel
	case "3", "error", "alert":
		lvl = zerolog.ErrorLevel
	case "4", "warn", "warning":
		lvl = zerolog.WarnLevel
	case "5", "info", "informational", "notice":
		lvl = zerolog.InfoLevel
	case "6", "debug":
		lvl = zerolog.DebugLevel
	case "7", "trace":
		lvl = zerolog.TraceLevel
	default:
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = time.RFC3339

	if (strings.EqualFold(logType, "off") || lvl == zerolog.Disabled) && bufferWriter == nil {
		Logger = zerolog.Nop()
		return
	}

	var writers []io.Writer
	if strings.EqualFold(logType, "stdout") || strings.EqualFold(logType, "both") {
		writers = append(writers,
			zerolog.ConsoleWriter{
				Out:        os.Stdout,
				TimeFormat: zerolog.TimeFieldFormat,
				NoColor:    !color,
			},
		)
	}
	if (strings.EqualFold(logType, "file") || strings.EqualFold(logType, "both")) &&
		logPath != "" &&
		!strings.EqualFold(logPath, "off") &&
		!strings.EqualFold(logPath, "false") &&
		!strings.EqualFold(logPath, "docker") &&
		logPath != "/dev/null" {

		lj := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    maxSize,
			MaxBackups: maxBackups,
			MaxAge:     maxAge,
			Compress:   compress,
			LocalTime:  true,
		}
		writers = append(writers, lj)
	}

	if bufferWriter != nil {
		writers = append(writers, bufferWriter)
	}

	multi := zerolog.MultiLevelWriter(writers...)
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}
	Logger = zerolog.New(multi).
		With().
		Timestamp().
		CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount + 1).
		Logger()
}

// Simple convenience methods
func Trace(msg string, v ...interface{}) { Logger.Trace().Msgf(msg, v...) }
func Debug(msg string, v ...interface{}) { Logger.Debug().Msgf(msg, v...) }
func Info(msg string, v ...interface{})  { Logger.Info().Msgf(msg, v...) }
func Warn(msg string, v ...interface{})  { Logger.Warn().Msgf(msg, v...) }
func Error(msg string, v ...interface{}) { Logger.Error().Msgf(msg, v...) }
func Fatal(msg string, v ...interface{}) { Logger.Fatal().Msgf(msg, v...) }
func Panic(msg string, v ...interface{}) { Logger.Panic().Msgf(msg, v...) }

// SetLevel updates the global minimum level
func SetLevel(levelStr string) {
	if lvl, err := zerolog.ParseLevel(strings.ToLower(levelStr)); err == nil {
		zerolog.SetGlobalLevel(lvl)
	}
}
