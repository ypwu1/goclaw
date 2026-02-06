package clawhub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// Default site URL
	DefaultSiteURL = "https://clawhub.ai"
	// Default registry API URL
	DefaultRegistryURL = "https://api.clawhub.ai"
	// Default skills directory
	DefaultSkillsDir = "skills"
	// Lockfile directory name
	LockfileDir = ".clawhub"
	// Lockfile name
	LockfileName = "lock.json"
	// Config file name
	ConfigFileName = "config.json"
)

// Config represents the clawhub configuration
type Config struct {
	SiteURL     string `json:"site_url"`
	RegistryURL string `json:"registry_url"`
	Token       string `json:"token,omitempty"`
	TokenLabel  string `json:"token_label,omitempty"`
	Workdir     string `json:"workdir,omitempty"`
	SkillsDir   string `json:"skills_dir,omitempty"`
}

// LoadConfig loads the configuration from the config directory
func LoadConfig() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	// If config doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return defaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves the configuration to the config directory
func SaveConfig(cfg *Config) error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// defaultConfig returns a default configuration
func defaultConfig() *Config {
	return &Config{
		SiteURL:     DefaultSiteURL,
		RegistryURL: DefaultRegistryURL,
		SkillsDir:   DefaultSkillsDir,
	}
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	// Check for custom config path from environment
	if customPath := os.Getenv("CLAWHUB_CONFIG_PATH"); customPath != "" {
		return customPath, nil
	}

	// Default to ~/.clawhub/config.json
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".clawhub", ConfigFileName), nil
}

// GetWorkdir returns the working directory
func (c *Config) GetWorkdir() (string, error) {
	// Check for workdir from environment
	if workdir := os.Getenv("CLAWHUB_WORKDIR"); workdir != "" {
		return workdir, nil
	}

	// Use configured workdir
	if c.Workdir != "" {
		return c.Workdir, nil
	}

	// Default to current directory
	return os.Getwd()
}

// GetSkillsDir returns the full path to the skills directory
func (c *Config) GetSkillsDir() (string, error) {
	workdir, err := c.GetWorkdir()
	if err != nil {
		return "", err
	}

	// Check if skills dir is absolute
	if filepath.IsAbs(c.SkillsDir) {
		return c.SkillsDir, nil
	}

	return filepath.Join(workdir, c.SkillsDir), nil
}

// GetLockfilePath returns the path to the lockfile
func (c *Config) GetLockfilePath() (string, error) {
	workdir, err := c.GetWorkdir()
	if err != nil {
		return "", err
	}

	return filepath.Join(workdir, LockfileDir, LockfileName), nil
}

// IsAuthenticated returns true if the user has a valid token
func (c *Config) IsAuthenticated() bool {
	return c.Token != ""
}

// SetToken sets the authentication token
func (c *Config) SetToken(token, label string) {
	c.Token = token
	if label != "" {
		c.TokenLabel = label
	} else {
		c.TokenLabel = "CLI token"
	}
}

// ClearToken removes the authentication token
func (c *Config) ClearToken() {
	c.Token = ""
	c.TokenLabel = ""
}

// GetSiteURL returns the site URL from config or environment
func GetSiteURL(cfg *Config) string {
	if siteURL := os.Getenv("CLAWHUB_SITE"); siteURL != "" {
		return siteURL
	}
	if cfg != nil && cfg.SiteURL != "" {
		return cfg.SiteURL
	}
	return DefaultSiteURL
}

// GetRegistryURL returns the registry URL from config or environment
func GetRegistryURL(cfg *Config) string {
	if registryURL := os.Getenv("CLAWHUB_REGISTRY"); registryURL != "" {
		return registryURL
	}
	if cfg != nil && cfg.RegistryURL != "" {
		return cfg.RegistryURL
	}
	return DefaultRegistryURL
}

// IsTelemetryDisabled returns true if telemetry is disabled
func IsTelemetryDisabled() bool {
	return os.Getenv("CLAWHUB_DISABLE_TELEMETRY") == "1"
}

// GetFallbackRoots returns the fallback scan roots for sync
func GetFallbackRoots() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	roots := []string{
		filepath.Join(homeDir, "openclaw", "skills"),
		filepath.Join(homeDir, ".openclaw", "skills"),
	}

	// Add platform-specific paths
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		roots = append(roots, filepath.Join(homeDir, ".goclaw", "skills"))
	}

	return roots, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.SiteURL == "" {
		return errors.New("site_url is required")
	}
	if c.RegistryURL == "" {
		return errors.New("registry_url is required")
	}
	if c.SkillsDir == "" {
		return errors.New("skills_dir is required")
	}
	return nil
}
