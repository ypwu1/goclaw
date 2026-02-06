package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smallnest/dogclaw/goclaw/clawhub"
	"github.com/spf13/cobra"
)

var (
	publishSlug      string
	publishName      string
	publishVersion   string
	publishChangelog string
	publishTags      string
)

var publishCmd = &cobra.Command{
	Use:   "publish <path>",
	Short: "Publish a skill to the registry",
	Long: `Publish a skill folder to the ClawHub registry.

Requires authentication. The skill folder must contain a SKILL.md file.`,
	Args: cobra.ExactArgs(1),
	Run:   runPublish,
}

var (
	syncRoot       []string
	syncAll        bool
	syncDryRun     bool
	syncBump       string
	syncChangelog  string
	syncTags       string
	syncConcurrency int
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local skills to the registry",
	Long: `Scan local skills and publish new/updated ones to the registry.

Scans your skills directory and publishes skills that are new or have changed
since the last published version.`,
	Run: runSync,
}

func addClawhubPublishCommands() {
	clawhubCmd.AddCommand(publishCmd)
	clawhubCmd.AddCommand(syncCmd)

	publishCmd.Flags().StringVar(&publishSlug, "slug", "", "Skill slug (required)")
	publishCmd.Flags().StringVar(&publishName, "name", "", "Display name (required)")
	publishCmd.Flags().StringVar(&publishVersion, "version", "", "Semver version (required)")
	publishCmd.Flags().StringVar(&publishChangelog, "changelog", "", "Changelog text")
	publishCmd.Flags().StringVar(&publishTags, "tags", "latest", "Comma-separated tags")

	publishCmd.MarkFlagRequired("slug")
	publishCmd.MarkFlagRequired("name")
	publishCmd.MarkFlagRequired("version")

	syncCmd.Flags().StringArrayVar(&syncRoot, "root", []string{}, "Extra scan roots")
	syncCmd.Flags().BoolVar(&syncAll, "all", false, "Upload everything without prompts")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show what would be uploaded without doing it")
	syncCmd.Flags().StringVar(&syncBump, "bump", "patch", "Auto-bump version: patch|minor|major")
	syncCmd.Flags().StringVar(&syncChangelog, "changelog", "", "Changelog for updates")
	syncCmd.Flags().StringVar(&syncTags, "tags", "latest", "Comma-separated tags")
	syncCmd.Flags().IntVar(&syncConcurrency, "concurrency", 4, "Concurrent registry checks")
}

func runPublish(cmd *cobra.Command, args []string) {
	skillPath := args[0]

	cfg, err := loadClawhubConfig()
	if err != nil {
		printError("Failed to load config: %v", err)
		os.Exit(1)
	}

	// Check authentication
	if err := requireAuth(cfg); err != nil {
		printError("%v", err)
		os.Exit(1)
	}

	// Validate slug
	if err := clawhub.ValidateSlug(publishSlug); err != nil {
		printError("Invalid slug: %v", err)
		os.Exit(1)
	}

	// Validate version
	if err := clawhub.ValidateVersion(publishVersion); err != nil {
		printError("Invalid version: %v", err)
		os.Exit(1)
	}

	// Resolve path
	absPath, err := filepath.Abs(skillPath)
	if err != nil {
		printError("Failed to resolve path: %v", err)
		os.Exit(1)
	}

	// Validate skill directory
	if err := clawhub.ValidateSkillDir(absPath); err != nil {
		printError("Invalid skill directory: %v", err)
		os.Exit(1)
	}

	// Create bundle
	printInfo("Creating bundle...")
	bundle, err := clawhub.CreateZipBundle(absPath)
	if err != nil {
		printError("Failed to create bundle: %v", err)
		os.Exit(1)
	}

	// Calculate hash
	hash, err := clawhub.CalculateHash(absPath)
	if err != nil {
		printError("Failed to calculate hash: %v", err)
		os.Exit(1)
	}

	printInfo("Bundle hash: %s", hash)

	// Parse tags
	tags := strings.Split(publishTags, ",")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}

	// Publish
	printInfo("Publishing %s@%s...", publishSlug, publishVersion)

	client := clawhub.NewClient(clawhub.GetRegistryURL(cfg), cfg.Token)

	req := &clawhub.PublishRequest{
		Slug:      publishSlug,
		Name:      publishName,
		Version:   publishVersion,
		Changelog: publishChangelog,
		Tags:      tags,
		Bundle:    bundle,
	}

	resp, err := client.Publish(req)
	if err != nil {
		printError("Failed to publish: %v", err)
		os.Exit(1)
	}

	printSuccess("Published %s@%s", resp.Slug, resp.Version)
	fmt.Printf("URL: %s\n", resp.URL)
}

func runSync(cmd *cobra.Command, args []string) {
	cfg, err := loadClawhubConfig()
	if err != nil {
		printError("Failed to load config: %v", err)
		os.Exit(1)
	}

	// Check authentication
	if err := requireAuth(cfg); err != nil {
		printError("%v", err)
		os.Exit(1)
	}

	// Collect scan roots
	skillsDir, err := cfg.GetSkillsDir()
	if err != nil {
		printError("Failed to get skills directory: %v", err)
		os.Exit(1)
	}

	roots := []string{skillsDir}
	roots = append(roots, syncRoot...)

	// Check if we found any skills
	skillDirs, err := clawhub.FindSkillDirectories(roots)
	if err != nil {
		printError("Failed to find skill directories: %v", err)
		os.Exit(1)
	}

	if len(skillDirs) == 0 {
		printInfo("No skills found in:")
		for _, root := range roots {
			fmt.Printf("  - %s\n", root)
		}

		// Try fallback roots
		fallbackRoots, err := clawhub.GetFallbackRoots()
		if err == nil {
			printInfo("Checking fallback locations...")
			skillDirs, err = clawhub.FindSkillDirectories(fallbackRoots)
			if err == nil && len(skillDirs) > 0 {
				printInfo("Found %d skills in fallback locations", len(skillDirs))
			}
		}

		if len(skillDirs) == 0 {
			printInfo("No skills found to sync")
			return
		}
	}

	printInfo("Found %d skill(s) to check", len(skillDirs))

	client := clawhub.NewClient(clawhub.GetRegistryURL(cfg), cfg.Token)

	// Parse tags
	tags := strings.Split(syncTags, ",")
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}

	published := 0
	skipped := 0
	failed := 0

	// Process each skill
	for _, skillDir := range skillDirs {
		slug := filepath.Base(skillDir)

		printInfo("\nChecking %s...", slug)

		// Get skill info from directory
		skillName, err := extractSkillName(skillDir)
		if err != nil {
			printError("Failed to extract skill name: %v", err)
			failed++
			continue
		}

		// Calculate current hash
		currentHash, err := clawhub.CalculateHash(skillDir)
		if err != nil {
			printError("Failed to calculate hash: %v", err)
			failed++
			continue
		}

		// Check registry for existing versions
		skillDetail, err := client.GetSkill(slug)
		isNew := false
		if err != nil {
			// Skill doesn't exist, will be new
			isNew = true
			skillDetail = &clawhub.SkillDetail{
				Slug:  slug,
				Name:  skillName,
				Tags:  tags,
				Versions: []clawhub.SkillVersion{},
			}
		}

		// Determine version
		var targetVersion string
		if isNew {
			targetVersion = "0.1.0"
		} else {
			// Check if hash matches any published version
			hashMatches := false
			for _, v := range skillDetail.Versions {
				if v.Hash == currentHash {
					hashMatches = true
					printInfo("Skill is already published at version %s", v.Version)
					break
				}
			}

			if hashMatches {
				skipped++
				continue
			}

			// Bump version
			if len(skillDetail.Versions) > 0 {
				latestVersion := skillDetail.Versions[0].Version
				targetVersion, err = clawhub.BumpVersion(latestVersion, syncBump)
				if err != nil {
					printError("Failed to bump version: %v", err)
					failed++
					continue
				}
			} else {
				targetVersion = "0.1.0"
			}
		}

		if syncDryRun {
			fmt.Printf("  Would publish: %s@%s\n", slug, targetVersion)
			published++
			continue
		}

		// Prompt if not --all
		if !syncAll && !confirm(fmt.Sprintf("Publish %s@%s?", slug, targetVersion)) {
			printInfo("Skipping %s", slug)
			skipped++
			continue
		}

		// Create bundle
		printInfo("Creating bundle...")
		bundle, err := clawhub.CreateZipBundle(skillDir)
		if err != nil {
			printError("Failed to create bundle: %v", err)
			failed++
			continue
		}

		// Publish
		printInfo("Publishing %s@%s...", slug, targetVersion)

		req := &clawhub.PublishRequest{
			Slug:      slug,
			Name:      skillName,
			Version:   targetVersion,
			Changelog: syncChangelog,
			Tags:      tags,
			Bundle:    bundle,
		}

		_, err = client.Publish(req)
		if err != nil {
			printError("Failed to publish: %v", err)
			failed++
			continue
		}

		printSuccess("Published %s@%s", slug, targetVersion)
		published++
	}

	// Summary
	fmt.Println()
	fmt.Printf("Sync summary: %d published, %d skipped, %d failed\n", published, skipped, failed)

	if !syncDryRun && !clawhub.IsTelemetryDisabled() {
		// Send minimal telemetry snapshot
		printInfo("Sending telemetry snapshot...")
		// TODO: Implement telemetry
	}
}

// extractSkillName extracts the skill name from SKILL.md
func extractSkillName(skillDir string) (string, error) {
	skillFile := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return "", err
	}

	// First line is typically the name
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			name := strings.TrimPrefix(line, "# ")
			name = strings.TrimSpace(name)
			if name != "" {
				return name, nil
			}
		}
	}

	// Fallback to directory name
	return filepath.Base(skillDir), nil
}
