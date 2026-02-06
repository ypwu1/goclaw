package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smallnest/dogclaw/goclaw/clawhub"
	"github.com/spf13/cobra"
)

var (
	installVersion string
	installForce   bool
)

var installCmd = &cobra.Command{
	Use:   "install <slug>",
	Short: "Install a skill from the registry",
	Long: `Install a skill from the ClawHub registry to your local skills directory.

Uses the latest version by default. Use --version to install a specific version.`,
	Args: cobra.ExactArgs(1),
	Run:   runInstall,
}

var (
	updateVersion string
	updateForce   bool
	updateAll     bool
)

var updateCmd = &cobra.Command{
	Use:   "update [slug]",
	Short: "Update installed skills",
	Long: `Update one or all installed skills to their latest versions.

Updates all installed skills with --all flag, or a specific skill if slug is provided.`,
	Run: runUpdate,
}

func addClawhubInstallCommands() {
	clawhubCmd.AddCommand(installCmd)
	clawhubCmd.AddCommand(updateCmd)

	installCmd.Flags().StringVar(&installVersion, "version", "", "Install a specific version")
	installCmd.Flags().BoolVar(&installForce, "force", false, "Overwrite if folder already exists")

	updateCmd.Flags().StringVar(&updateVersion, "version", "", "Update to specific version (single slug only)")
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "Overwrite when local files don't match any published version")
	updateCmd.Flags().BoolVar(&updateAll, "all", false, "Update all installed skills")
}

func runInstall(cmd *cobra.Command, args []string) {
	slug := args[0]

	// Validate slug
	if err := clawhub.ValidateSlug(slug); err != nil {
		printError("Invalid slug: %v", err)
		os.Exit(1)
	}

	cfg, err := loadClawhubConfig()
	if err != nil {
		printError("Failed to load config: %v", err)
		os.Exit(1)
	}

	client := clawhub.NewClient(clawhub.GetRegistryURL(cfg), cfg.Token)

	// Get skill details
	skillDetail, err := client.GetSkill(slug)
	if err != nil {
		printError("Failed to get skill: %v", err)
		os.Exit(1)
	}

	// Determine version to install
	version := installVersion
	if version == "" {
		if len(skillDetail.Versions) == 0 {
			printError("No versions available for skill '%s'", slug)
			os.Exit(1)
		}
		// Get latest version
		version = skillDetail.Versions[0].Version
	}

	// Verify version exists
	versionExists := false
	for _, v := range skillDetail.Versions {
		if v.Version == version {
			versionExists = true
			break
		}
	}

	if !versionExists {
		printError("Version %s not found. Available versions:", version)
		for _, v := range skillDetail.Versions {
			fmt.Printf("  - %s\n", v.Version)
		}
		os.Exit(1)
	}

	// Get skills directory
	skillsDir, err := cfg.GetSkillsDir()
	if err != nil {
		printError("Failed to get skills directory: %v", err)
		os.Exit(1)
	}

	// Create skills directory if needed
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		printError("Failed to create skills directory: %v", err)
		os.Exit(1)
	}

	// Check if skill already exists
	skillPath := filepath.Join(skillsDir, slug)
	if _, err := os.Stat(skillPath); err == nil {
		if !installForce && !confirm(fmt.Sprintf("Skill '%s' already exists. Overwrite?", slug)) {
			printInfo("Installation cancelled")
			return
		}
		// Remove existing skill
		if err := os.RemoveAll(skillPath); err != nil {
			printError("Failed to remove existing skill: %v", err)
			os.Exit(1)
		}
	}

	// Download skill
	printInfo("Downloading %s@%s...", slug, version)
	data, err := client.DownloadSkill(slug, version)
	if err != nil {
		printError("Failed to download skill: %v", err)
		os.Exit(1)
	}

	// Extract skill
	printInfo("Extracting to %s...", skillPath)
	if err := clawhub.ExtractZipBundle(data, skillPath); err != nil {
		printError("Failed to extract skill: %v", err)
		os.Exit(1)
	}

	// Get version hash
	var hash string
	for _, v := range skillDetail.Versions {
		if v.Version == version {
			hash = v.Hash
			break
		}
	}

	// Update lockfile
	workdir, err := cfg.GetWorkdir()
	if err != nil {
		printError("Failed to get workdir: %v", err)
		os.Exit(1)
	}

	lockfile, err := clawhub.LoadLockfile(workdir)
	if err != nil {
		printError("Failed to load lockfile: %v", err)
		os.Exit(1)
	}

	lockfile.AddSkill(slug, skillDetail.Name, version, hash, skillDetail.Tags)
	if err := lockfile.Save(workdir); err != nil {
		printError("Failed to save lockfile: %v", err)
		os.Exit(1)
	}

	printSuccess("Installed %s@%s", slug, version)
	fmt.Println("\nStart a new goclaw session to use this skill.")
}

func runUpdate(cmd *cobra.Command, args []string) {
	cfg, err := loadClawhubConfig()
	if err != nil {
		printError("Failed to load config: %v", err)
		os.Exit(1)
	}

	client := clawhub.NewClient(clawhub.GetRegistryURL(cfg), cfg.Token)

	workdir, err := cfg.GetWorkdir()
	if err != nil {
		printError("Failed to get workdir: %v", err)
		os.Exit(1)
	}

	lockfile, err := clawhub.LoadLockfile(workdir)
	if err != nil {
		printError("Failed to load lockfile: %v", err)
		os.Exit(1)
	}

	if lockfile.SkillCount() == 0 {
		printInfo("No skills installed")
		return
	}

	if updateAll {
		// Update all skills
		updateAllSkills(cfg, client, workdir, lockfile)
	} else {
		// Update specific skill
		if len(args) == 0 {
			printError("Please provide a skill slug or use --all to update all skills")
			os.Exit(1)
		}
		slug := args[0]
		updateSingleSkill(slug, cfg, client, workdir, lockfile)
	}
}

func updateAllSkills(cfg *clawhub.Config, client *clawhub.Client, workdir string, lockfile *clawhub.Lockfile) {
	updated := 0
	skipped := 0
	failed := 0

	for slug := range lockfile.ListSkills() {
		if err := updateSingleSkill(slug, cfg, client, workdir, lockfile); err != nil {
			printError("Failed to update %s: %v", slug, err)
			failed++
		} else {
			updated++
		}
	}

	fmt.Println()
	fmt.Printf("Update summary: %d updated, %d skipped, %d failed\n", updated, skipped, failed)
}

func updateSingleSkill(slug string, cfg *clawhub.Config, client *clawhub.Client, workdir string, lockfile *clawhub.Lockfile) error {
	// Get current version
	currentVersion, ok := lockfile.GetSkillVersion(slug)
	if !ok {
		return fmt.Errorf("skill '%s' not in lockfile", slug)
	}

	// Get latest info from registry
	skillDetail, err := client.GetSkill(slug)
	if err != nil {
		return err
	}

	// Determine target version
	targetVersion := updateVersion
	if targetVersion == "" {
		if len(skillDetail.Versions) == 0 {
			return fmt.Errorf("no versions available")
		}
		targetVersion = skillDetail.Versions[0].Version
	}

	// Check if update is needed
	if targetVersion == currentVersion {
		printInfo("%s is already at latest version %s", slug, currentVersion)
		return nil
	}

	// Compare versions
	if cmp, err := clawhub.CompareVersions(currentVersion, targetVersion); err == nil && cmp >= 0 {
		printInfo("%s is already up to date (%s >= %s)", slug, currentVersion, targetVersion)
		return nil
	}

	// Check for local changes
	skillsDir, err := cfg.GetSkillsDir()
	if err != nil {
		return err
	}

	skillPath := filepath.Join(skillsDir, slug)
	currentHash, _ := lockfile.GetSkillHash(slug)
	diskHash, err := clawhub.CalculateHash(skillPath)
	if err == nil && currentHash != diskHash && !updateForce {
		if !confirm(fmt.Sprintf("Local changes detected in %s. Overwrite?", slug)) {
			printInfo("Skipping %s", slug)
			return nil
		}
	}

	// Download and install
	printInfo("Updating %s from %s to %s...", slug, currentVersion, targetVersion)
	data, err := client.DownloadSkill(slug, targetVersion)
	if err != nil {
		return err
	}

	// Remove existing skill
	if err := os.RemoveAll(skillPath); err != nil {
		return fmt.Errorf("failed to remove existing skill: %w", err)
	}

	// Extract new version
	if err := clawhub.ExtractZipBundle(data, skillPath); err != nil {
		return err
	}

	// Get version hash
	var hash string
	for _, v := range skillDetail.Versions {
		if v.Version == targetVersion {
			hash = v.Hash
			break
		}
	}

	// Update lockfile
	lockfile.UpdateSkillVersion(slug, targetVersion, hash, skillDetail.Tags)
	if err := lockfile.Save(workdir); err != nil {
		return err
	}

	printSuccess("Updated %s to %s", slug, targetVersion)
	return nil
}
