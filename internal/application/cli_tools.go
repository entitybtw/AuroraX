package app

import (
	"aurora/configuration"
	"aurora/internal/command_line_tools"
)

func cliToolsService(cfg *config.Config) *clitools.Service {
	if cfg == nil || !cfg.CLITools.Enabled {
		return nil
	}
	return clitools.NewService(cfg.CLITools.ApplyEnabled, nil)
}
