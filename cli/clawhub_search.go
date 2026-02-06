package cli

import (
	"fmt"
	"os"

	"github.com/smallnest/dogclaw/goclaw/clawhub"
	"github.com/spf13/cobra"
)

var (
	searchLimit int
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for skills",
	Long: `Search for skills in the ClawHub registry using vector search.
Not just keyword matching - understands natural language queries.`,
	Args: cobra.ExactArgs(1),
	Run:   runSearch,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed skills",
	Long:  `Display all installed skills from the lockfile.`,
	Run:   runList,
}

func addClawhubSearchCommands() {
	clawhubCmd.AddCommand(searchCmd)
	clawhubCmd.AddCommand(listCmd)

	searchCmd.Flags().IntVar(&searchLimit, "limit", 10, "Maximum number of results to display")
}

func runSearch(cmd *cobra.Command, args []string) {
	query := args[0]

	client, err := getClawhubClient()
	if err != nil {
		printError("Failed to create client: %v", err)
		os.Exit(1)
	}

	results, err := client.Search(query, searchLimit)
	if err != nil {
		printError("Search failed: %v", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		printInfo("No skills found for: %s", query)
		return
	}

	fmt.Printf("Found %d skills:\n\n", len(results))
	for i, result := range results {
		fmt.Printf("[%d] %s %s\n", i+1, result.Slug, result.Name)
		fmt.Printf("    ⭐ %d stars | ⤓ %d downloads | ⤒ %d updates\n",
			result.Stats.Stars, result.Stats.Downloads, result.Stats.Updates)
		if len(result.Tags) > 0 {
			fmt.Printf("    Tags: %s\n", formatTags(result.Tags))
		}
		if result.Description != "" {
			fmt.Printf("    %s\n", result.Description)
		}
		fmt.Println()
	}
}

func runList(cmd *cobra.Command, args []string) {
	cfg, err := loadClawhubConfig()
	if err != nil {
		printError("Failed to load config: %v", err)
		os.Exit(1)
	}

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
		fmt.Println("\nInstall a skill with: goclaw clawhub install <slug>")
		return
	}

	fmt.Println("Installed Skills:")
	fmt.Println("=================")
	for slug, skill := range lockfile.ListSkills() {
		fmt.Printf("[%s] %s - %s\n", slug, skill.Version, skill.Name)
		if len(skill.Tags) > 0 {
			fmt.Printf("    Tags: %s\n", formatTags(skill.Tags))
		}
	}
}

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	result := ""
	for i, tag := range tags {
		if i > 0 {
			result += ", "
		}
		result += tag
	}
	return result
}
