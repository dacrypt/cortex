package main

import (
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
	"github.com/rs/zerolog"
)

// initializeTikaManager initializes Tika Manager if Tika is enabled and process management is enabled.
func initializeTikaManager(cfg *config.Config, logger zerolog.Logger) *metadata.TikaManager {
	if !cfg.Tika.Enabled || !cfg.Tika.ManageProcess {
		return nil
	}

	// Check if Java is available
	if err := metadata.CheckJava(); err != nil {
		logger.Warn().
			Err(err).
			Msg("Java not found - Tika Server cannot be managed automatically. Install Java or set tika.manage_process: false")
		return nil
	}

	managerConfig := metadata.TikaManagerConfig{
		Enabled:        cfg.Tika.ManageProcess,
		JarPath:        cfg.Tika.JarPath,
		DataDir:        cfg.DataDir, // Use Cortex data directory
		AutoDownload:   cfg.Tika.AutoDownload,
		Endpoint:       cfg.Tika.Endpoint,
		Port:           cfg.Tika.Port,
		StartupTimeout: cfg.Tika.StartupTimeout,
		HealthInterval: cfg.Tika.HealthInterval,
		MaxRestarts:    cfg.Tika.MaxRestarts,
		RestartDelay:   cfg.Tika.RestartDelay,
	}

	manager := metadata.NewTikaManager(managerConfig, logger)
	return manager
}

