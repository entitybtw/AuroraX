package clitools

type Tool struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Description    string       `json:"description"`
	ConfigPath     string       `json:"config_path,omitempty"`
	CanApply       bool         `json:"can_apply"`
	ConfigType     string       `json:"config_type,omitempty"`
	Color          string       `json:"color,omitempty"`
	DocsURL        string       `json:"docs_url,omitempty"`
	DefaultCommand string       `json:"default_command,omitempty"`
	Notes          []string     `json:"notes,omitempty"`
	ModelFields    []ModelField `json:"model_fields,omitempty"`
}

type ModelField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Description  string `json:"description,omitempty"`
	DefaultModel string `json:"default_model,omitempty"`
	Multi        bool   `json:"multi,omitempty"`
}

type PreviewRequest struct {
	BaseURL        string            `json:"base_url"`
	APIKey         string            `json:"api_key"`
	Model          string            `json:"model"`
	ModelOverrides map[string]string `json:"model_overrides,omitempty"`
	Models         []string          `json:"models,omitempty"`
}

type PreviewResponse struct {
	Tool      Tool              `json:"tool"`
	Snippets  map[string]string `json:"snippets"`
	MaskedKey string            `json:"masked_key,omitempty"`
}

type ApplyResponse struct {
	Applied    bool   `json:"applied"`
	Path       string `json:"path"`
	BackupPath string `json:"backup_path,omitempty"`
}
