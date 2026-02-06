package cli

import (
	"fmt"
	"os"

	"github.com/smallnest/dogclaw/goclaw/clawhub"
	"github.com/spf13/cobra"
)

var clawhubCmd = &cobra.Command{
	Use:   "clawhub",
	Short: "ClawHub skill registry commands",
	Long: `ClawHub is the public skill registry for OpenClaw.
Search, install, update, and publish skills.`,
}

// Global flags for clawhub
var (
	clawhubWorkdir   string
	clawhubDir       string
	clawhubSite      string
	clawhubRegistry  string
	clawhubNoInput   bool
	clawhubCLIConfig *clawhub.Config
)

func init() {
	rootCmd.AddCommand(clawhubCmd)

	// Global flags
	clawhubCmd.PersistentFlags().StringVar(&clawhubWorkdir, "workdir", "", "Working directory (default: current dir; falls back to OpenClaw workspace)")
	clawhubCmd.PersistentFlags().StringVar(&clawhubDir, "dir", "skills", "Skills directory, relative to workdir")
	clawhubCmd.PersistentFlags().StringVar(&clawhubSite, "site", "", "Site base URL (browser login)")
	clawhubCmd.PersistentFlags().StringVar(&clawhubRegistry, "registry", "", "Registry API base URL")
	clawhubCmd.PersistentFlags().BoolVar(&clawhubNoInput, "no-input", false, "Disable prompts (non-interactive)")

	// Add subcommands
	addClawhubAuthCommands()
	addClawhubSearchCommands()
	addClawhubInstallCommands()
	addClawhubPublishCommands()
	addClawhubAdminCommands()
}

// loadClawhubConfig loads the clawhub configuration
func loadClawhubConfig() (*clawhub.Config, error) {
	cfg, err := clawhub.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Apply command-line overrides
	if clawhubSite != "" {
		cfg.SiteURL = clawhubSite
	}
	if clawhubRegistry != "" {
		cfg.RegistryURL = clawhubRegistry
	}
	if clawhubWorkdir != "" {
		cfg.Workdir = clawhubWorkdir
	}
	if clawhubDir != "" {
		cfg.SkillsDir = clawhubDir
	}

	clawhubCLIConfig = cfg
	return cfg, nil
}

// getClawhubClient creates a new registry client
func getClawhubClient() (*clawhub.Client, error) {
	cfg, err := loadClawhubConfig()
	if err != nil {
		return nil, err
	}

	registryURL := clawhub.GetRegistryURL(cfg)
	return clawhub.NewClient(registryURL, cfg.Token), nil
}

// requireAuth checks if the user is authenticated
func requireAuth(cfg *clawhub.Config) error {
	if !cfg.IsAuthenticated() {
		return fmt.Errorf("not logged in. Run 'goclaw clawhub login' first")
	}
	return nil
}

// confirm prompts the user for confirmation (unless --no-input is set)
func confirm(message string) bool {
	if clawhubNoInput {
		return false
	}

	fmt.Printf("%s (y/N): ", message)
	var response string
	fmt.Scanln(&response)
	return response == "y" || response == "Y"
}

// prompt prompts the user for input (unless --no-input is set)
func prompt(message string) string {
	if clawhubNoInput {
		return ""
	}

	fmt.Printf("%s: ", message)
	var response string
	fmt.Scanln(&response)
	return response
}

// printSuccess prints a success message
func printSuccess(format string, args ...interface{}) {
	fmt.Printf("✅ %s\n", fmt.Sprintf(format, args...))
}

// printError prints an error message
func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ %s\n", fmt.Sprintf(format, args...))
}

// printWarning prints a warning message
func printWarning(format string, args ...interface{}) {
	fmt.Printf("⚠️  %s\n", fmt.Sprintf(format, args...))
}

// printInfo prints an info message
func printInfo(format string, args ...interface{}) {
	fmt.Printf("ℹ️  %s\n", fmt.Sprintf(format, args...))
}
