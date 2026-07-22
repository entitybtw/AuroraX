package model_combinations

type ComboDefinition struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description,omitempty"`
	Models      []string `yaml:"models" json:"models"`
	Enabled     *bool    `yaml:"enabled" json:"enabled,omitempty"`
}

type CombosConfig struct {
	Enabled     bool              `yaml:"enabled" env:"COMBOS_ENABLED"`
	Definitions []ComboDefinition `yaml:"definitions"`
}
