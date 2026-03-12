package providerslog

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Official-Husko/pkmn-tc-value/internal/store"
)

var nonFileCharsRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type Logger struct {
	enabled bool
	baseDir string

	mu      sync.Mutex
	counter uint64
}

func New(enabled bool, baseDir string) *Logger {
	logger := &Logger{
		enabled: enabled,
		baseDir: strings.TrimSpace(baseDir),
	}
	if logger.enabled && logger.baseDir != "" {
		_ = os.MkdirAll(logger.baseDir, 0o755)
	}
	return logger
}

func (l *Logger) LogJSON(provider string, endpoint string, body []byte) {
	if l == nil || !l.enabled {
		return
	}
	if len(body) == 0 || !json.Valid(body) {
		return
	}
	l.write(provider, endpoint, "json", body)
}

func (l *Logger) LogHTTP(provider string, endpoint string, statusCode int, status string, body []byte) {
	if l == nil || !l.enabled {
		return
	}

	record := map[string]any{
		"url":        strings.TrimSpace(endpoint),
		"statusCode": statusCode,
		"status":     strings.TrimSpace(status),
	}

	if len(body) > 0 {
		if json.Valid(body) {
			raw := make(json.RawMessage, len(body))
			copy(raw, body)
			record["json"] = raw
		} else {
			record["body"] = string(body)
		}
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return
	}
	l.write(provider, endpoint, fmt.Sprintf("http_%03d", statusCode), payload)
}

func (l *Logger) write(provider string, endpoint string, suffix string, body []byte) {
	base := strings.TrimSpace(l.baseDir)
	if base == "" || len(body) == 0 {
		return
	}

	providerName := sanitize(provider)
	if providerName == "" {
		providerName = "provider"
	}
	dir := filepath.Join(base, providerName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	sum := sha1.Sum([]byte(endpoint))
	hash := hex.EncodeToString(sum[:])[:10]
	now := time.Now().UTC()
	l.mu.Lock()
	l.counter++
	seq := l.counter
	l.mu.Unlock()

	endpointName := sanitize(endpoint)
	if endpointName == "" {
		endpointName = "response"
	}
	if len(endpointName) > 72 {
		endpointName = endpointName[:72]
	}

	name := fmt.Sprintf("%s_%06d_%s_%s", now.Format("20060102T150405.000Z"), seq, endpointName, hash)
	if trimmed := sanitize(suffix); trimmed != "" {
		name += "_" + trimmed
	}
	name += ".json"

	_ = store.WriteFileAtomically(filepath.Join(dir, name), body, 0o600)
}

func sanitize(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}
	clean := nonFileCharsRE.ReplaceAllString(trimmed, "_")
	clean = strings.Trim(clean, "._-")
	return clean
}
