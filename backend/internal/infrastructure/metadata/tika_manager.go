package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// TikaManager manages the lifecycle of Tika Server process.
type TikaManager struct {
	cmd        *exec.Cmd
	config     TikaManagerConfig
	logger     zerolog.Logger
	mu         sync.RWMutex
	isRunning  bool
	stopCh     chan struct{}
	healthCh   chan bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// TikaManagerConfig holds configuration for Tika Manager.
type TikaManagerConfig struct {
	Enabled        bool
	JarPath        string // Path to tika-server-standard.jar (empty = auto-detect or download)
	DataDir        string // Directory to store downloaded JAR
	AutoDownload   bool   // Automatically download JAR if not found
	Endpoint       string // http://localhost:9998
	Port           int    // Port to run Tika on
	StartupTimeout time.Duration
	HealthInterval time.Duration
	MaxRestarts    int
	RestartDelay   time.Duration
}

// DefaultTikaManagerConfig returns default configuration.
func DefaultTikaManagerConfig() TikaManagerConfig {
	return TikaManagerConfig{
		Enabled:        true,
		JarPath:        "", // Auto-detect or download
		DataDir:        "", // Will be set from config
		AutoDownload:   true,
		Endpoint:       "http://localhost:9998",
		Port:           9998,
		StartupTimeout: 30 * time.Second,
		HealthInterval: 10 * time.Second,
		MaxRestarts:    3,
		RestartDelay:   5 * time.Second,
	}
}

// NewTikaManager creates a new Tika Manager.
func NewTikaManager(config TikaManagerConfig, logger zerolog.Logger) *TikaManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TikaManager{
		config:     config,
		logger:     logger.With().Str("component", "tika_manager").Logger(),
		stopCh:     make(chan struct{}),
		healthCh:   make(chan bool, 1),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts Tika Server process.
func (m *TikaManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return fmt.Errorf("tika manager is already running")
	}

	if !m.config.Enabled {
		m.logger.Info().Msg("Tika Manager is disabled")
		return nil
	}

	// Find Tika JAR
	jarPath, err := m.findTikaJar()
	if err != nil {
		return fmt.Errorf("failed to find Tika JAR: %w", err)
	}

	m.logger.Info().
		Str("jar", jarPath).
		Int("port", m.config.Port).
		Msg("Starting Tika Server")

	// Build command
	cmd := exec.CommandContext(ctx, "java", "-jar", jarPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", m.config.Port))

	// Set up logging
	cmd.Stdout = &logWriter{logger: m.logger, level: "info"}
	cmd.Stderr = &logWriter{logger: m.logger, level: "error"}

	// Start process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Tika Server: %w", err)
	}

	m.cmd = cmd
	m.isRunning = true

	// Start monitoring goroutine
	go m.monitor()

	// Wait for Tika to be ready
	if err := m.waitForReady(ctx); err != nil {
		m.Stop()
		return fmt.Errorf("tika server failed to start: %w", err)
	}

	m.logger.Info().Msg("Tika Server started successfully")
	return nil
}

// Stop stops Tika Server process.
func (m *TikaManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	m.logger.Info().Msg("Stopping Tika Server")

	// Cancel context to stop monitoring
	m.cancel()

	// Signal stop
	close(m.stopCh)

	// Stop process
	if m.cmd != nil && m.cmd.Process != nil {
		// Try graceful shutdown first
		if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
			m.logger.Warn().Err(err).Msg("Failed to send interrupt signal, killing process")
			if err := m.cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill Tika process: %w", err)
			}
		} else {
			// Wait for graceful shutdown (5 seconds)
			done := make(chan error, 1)
			go func() {
				done <- m.cmd.Wait()
			}()

			select {
			case <-time.After(5 * time.Second):
				m.logger.Warn().Msg("Tika did not stop gracefully, killing process")
				if err := m.cmd.Process.Kill(); err != nil {
					return fmt.Errorf("failed to kill Tika process: %w", err)
				}
			case err := <-done:
				if err != nil {
					m.logger.Debug().Err(err).Msg("Tika process exited")
				}
			}
		}
	}

	m.isRunning = false
	m.logger.Info().Msg("Tika Server stopped")
	return nil
}

// IsRunning returns true if Tika Server is running.
func (m *TikaManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// GetEndpoint returns the Tika Server endpoint.
func (m *TikaManager) GetEndpoint() string {
	return m.config.Endpoint
}

// monitor monitors Tika Server health and restarts if needed.
func (m *TikaManager) monitor() {
	ticker := time.NewTicker(m.config.HealthInterval)
	defer ticker.Stop()

	restartCount := 0

	for {
		select {
		case <-m.stopCh:
			return
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if !m.isRunning {
				continue
			}

			// Check if process is still alive
			if m.cmd != nil && m.cmd.Process != nil {
				if err := m.cmd.Process.Signal(os.Signal(nil)); err != nil {
					m.logger.Warn().Err(err).Msg("Tika process is not running")
					
					if restartCount < m.config.MaxRestarts {
						restartCount++
						m.logger.Info().
							Int("restart_count", restartCount).
							Int("max_restarts", m.config.MaxRestarts).
							Msg("Attempting to restart Tika Server")
						
						time.Sleep(m.config.RestartDelay)
						if err := m.restart(); err != nil {
							m.logger.Error().Err(err).Msg("Failed to restart Tika Server")
						} else {
							restartCount = 0 // Reset on successful restart
						}
					} else {
						m.logger.Error().
							Int("max_restarts", m.config.MaxRestarts).
							Msg("Max restart attempts reached, Tika Server will not be restarted")
						m.isRunning = false
						return
					}
				}
			}

			// Health check via HTTP
			client := NewTikaClient(m.config.Endpoint, 5*time.Second, m.logger)
			if err := client.HealthCheck(context.Background()); err != nil {
				m.logger.Warn().Err(err).Msg("Tika Server health check failed")
			}
		}
	}
}

// restart restarts Tika Server.
func (m *TikaManager) restart() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Kill()
		m.cmd.Wait()
	}

	m.isRunning = false

	// Start again
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	jarPath, err := m.findTikaJar()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "java", "-jar", jarPath,
		"--host", "0.0.0.0",
		"--port", fmt.Sprintf("%d", m.config.Port))

	cmd.Stdout = &logWriter{logger: m.logger, level: "info"}
	cmd.Stderr = &logWriter{logger: m.logger, level: "error"}

	if err := cmd.Start(); err != nil {
		return err
	}

	m.cmd = cmd
	m.isRunning = true

	// Wait for ready
	readyCtx, readyCancel := context.WithTimeout(context.Background(), m.config.StartupTimeout)
	defer readyCancel()

	if err := m.waitForReady(readyCtx); err != nil {
		m.isRunning = false
		return err
	}

	return nil
}

// waitForReady waits for Tika Server to be ready.
func (m *TikaManager) waitForReady(ctx context.Context) error {
	client := NewTikaClient(m.config.Endpoint, 5*time.Second, m.logger)
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.NewTimer(m.config.StartupTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for Tika Server to be ready")
		case <-ticker.C:
			if err := client.HealthCheck(ctx); err == nil {
				return nil
			}
		}
	}
}

// findTikaJar finds the Tika Server JAR file.
func (m *TikaManager) findTikaJar() (string, error) {
	// If explicit path provided, use it
	if m.config.JarPath != "" {
		if _, err := os.Stat(m.config.JarPath); err == nil {
			return m.config.JarPath, nil
		}
		return "", fmt.Errorf("tika JAR not found at %s", m.config.JarPath)
	}

	// Try to find Tika in common locations
	searchPaths := []string{
		// System-wide locations
		"/usr/share/tika/tika-server-standard.jar",
		"/usr/local/share/tika/tika-server-standard.jar",
		"/opt/tika/tika-server-standard.jar",
		
		// Homebrew (macOS)
		"/opt/homebrew/Cellar/tika/tika-server-standard.jar",
		"/usr/local/Cellar/tika/tika-server-standard.jar",
		
		// Current directory
		"./tika-server-standard.jar",
		"./tika/tika-server-standard.jar",
	}

	// Also check if 'tika-server' command is available (wrapper script)
	if path, err := exec.LookPath("tika-server"); err == nil {
		// Check if it's a script that points to a JAR
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// Try to extract JAR path from script or use it directly
			// For now, just add the directory to search
			dir := filepath.Dir(path)
			searchPaths = append([]string{filepath.Join(dir, "tika-server-standard.jar")}, searchPaths...)
		}
	}

	// Check JAVA_HOME for bundled Tika
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		searchPaths = append([]string{
			filepath.Join(javaHome, "lib", "tika-server-standard.jar"),
		}, searchPaths...)
	}

	// Check each path
	for _, path := range searchPaths {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			m.logger.Debug().Str("path", path).Msg("Found Tika JAR")
			return path, nil
		}
	}

	// Last resort: try to download if auto-download is enabled
	if m.config.AutoDownload {
		m.logger.Info().Msg("Tika JAR not found, attempting to download...")
		jarPath, err := m.downloadTikaJar()
		if err == nil {
			return jarPath, nil
		}
		m.logger.Warn().Err(err).Msg("Failed to download Tika JAR")
	}

	// If download failed or disabled, return error with instructions
	return "", fmt.Errorf(
		"Tika Server JAR not found. Please:\n" +
			"1. Set tika.auto_download: true to download automatically\n" +
			"2. Set tika.jar_path in configuration to point to tika-server-standard.jar\n" +
			"3. Install Tika: brew install tika (macOS) or download from https://tika.apache.org/download.html")
}

// downloadTikaJar downloads Tika Server JAR from Apache Maven repository.
func (m *TikaManager) downloadTikaJar() (string, error) {
	if m.config.DataDir == "" {
		return "", fmt.Errorf("data directory not configured for Tika JAR download")
	}

	// Create Tika directory in data dir
	tikaDir := filepath.Join(m.config.DataDir, "tika")
	if err := os.MkdirAll(tikaDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create Tika directory: %w", err)
	}

	jarPath := filepath.Join(tikaDir, "tika-server-standard.jar")

	// Check if already downloaded
	if info, err := os.Stat(jarPath); err == nil && !info.IsDir() {
		m.logger.Info().Str("path", jarPath).Msg("Tika JAR already downloaded")
		return jarPath, nil
	}

	// Download from Apache Maven repository
	// Using latest stable version (2.9.1 as of writing)
	version := "2.9.1"
	url := fmt.Sprintf("https://archive.apache.org/dist/tika/tika-server-standard-%s.jar", version)

	m.logger.Info().
		Str("url", url).
		Str("destination", jarPath).
		Msg("Downloading Tika Server JAR")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Download file
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download Tika JAR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download Tika JAR: HTTP %d", resp.StatusCode)
	}

	// Create file
	file, err := os.Create(jarPath)
	if err != nil {
		return "", fmt.Errorf("failed to create JAR file: %w", err)
	}
	defer file.Close()

	// Download with progress (optional: could add progress callback)
	hasher := sha256.New()
	multiWriter := io.MultiWriter(file, hasher)

	written, err := io.Copy(multiWriter, resp.Body)
	if err != nil {
		os.Remove(jarPath) // Clean up on error
		return "", fmt.Errorf("failed to write JAR file: %w", err)
	}

	// Verify checksum (optional but recommended)
	// For now, we'll just log the hash for verification
	hash := hex.EncodeToString(hasher.Sum(nil))
	m.logger.Info().
		Str("path", jarPath).
		Int64("size", written).
		Str("sha256", hash).
		Msg("Tika JAR downloaded successfully")

	// Make executable (not needed for JAR but good practice)
	if err := os.Chmod(jarPath, 0644); err != nil {
		m.logger.Warn().Err(err).Msg("Failed to set JAR permissions")
	}

	return jarPath, nil
}

// logWriter writes command output to logger.
type logWriter struct {
	logger zerolog.Logger
	level  string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	line := string(p)
	switch w.level {
	case "error":
		w.logger.Error().Str("tika", line).Msg("")
	case "info":
		w.logger.Info().Str("tika", line).Msg("")
	default:
		w.logger.Debug().Str("tika", line).Msg("")
	}
	return len(p), nil
}

// CheckJava checks if Java is available.
func CheckJava() error {
	cmd := exec.Command("java", "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Java is not installed or not in PATH. Tika Server requires Java")
	}
	return nil
}

