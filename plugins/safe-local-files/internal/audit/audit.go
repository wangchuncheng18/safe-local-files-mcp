package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Time       string `json:"time"`
	Event      string `json:"event"`
	Tool       string `json:"tool,omitempty"`
	Path       string `json:"path,omitempty"`
	Outcome    string `json:"outcome"`
	Bytes      int    `json:"bytes,omitempty"`
	Results    int    `json:"results,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	RemoteHash string `json:"remote_hash,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

type Logger struct {
	path string
	mu   sync.Mutex
}

func New(path string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create audit directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return &Logger{path: path}, nil
}

func (l *Logger) Write(e Event) {
	e.Time = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
