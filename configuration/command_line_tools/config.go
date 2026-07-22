package command_line_tools

type CLIToolsConfig struct {
	Enabled      bool `yaml:"enabled" env:"CLI_TOOLS_ENABLED"`
	ApplyEnabled bool `yaml:"apply_enabled" env:"CLI_TOOLS_APPLY_ENABLED"`
}
