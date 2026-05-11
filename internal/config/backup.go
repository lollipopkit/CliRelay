package config

// BackupConfig controls automatic and manual backup behavior.
type BackupConfig struct {
	Enable           bool   `yaml:"enable" json:"enable"`
	Dir              string `yaml:"dir,omitempty" json:"dir,omitempty"`
	Cron             string `yaml:"cron,omitempty" json:"cron,omitempty"`
	MaxKeep          int    `yaml:"max-keep,omitempty" json:"max-keep,omitempty"`
	MaxAgeDays       int    `yaml:"max-age-days,omitempty" json:"max-age-days,omitempty"`
	IncludeAuthFiles bool   `yaml:"include-auth-files,omitempty" json:"include-auth-files,omitempty"`
}
