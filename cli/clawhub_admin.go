package cli

import (
	"fmt"
	"os"

	"github.com/smallnest/dogclaw/goclaw/clawhub"
	"github.com/spf13/cobra"
)

var (
	deleteYes bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Delete a skill from the registry",
	Long: `Delete a skill from the ClawHub registry.

Only the skill owner or admin can delete a skill. Use --yes to skip confirmation.`,
	Args: cobra.ExactArgs(1),
	Run:   runDelete,
}

var undeleteCmd = &cobra.Command{
	Use:   "undelete <slug>",
	Short: "Undelete a skill from the registry",
	Long: `Undelete a previously deleted skill from the ClawHub registry.

Only the skill owner or admin can undelete a skill. Use --yes to skip confirmation.`,
	Args: cobra.ExactArgs(1),
	Run:   runUndelete,
}

func addClawhubAdminCommands() {
	clawhubCmd.AddCommand(deleteCmd)
	clawhubCmd.AddCommand(undeleteCmd)

	deleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "Skip confirmation prompt")
	undeleteCmd.Flags().BoolVar(&deleteYes, "yes", false, "Skip confirmation prompt")
}

func runDelete(cmd *cobra.Command, args []string) {
	slug := args[0]

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

	// Confirm deletion
	if !deleteYes && !confirm(fmt.Sprintf("Delete skill '%s' from the registry?", slug)) {
		printInfo("Deletion cancelled")
		return
	}

	// Delete skill
	client := clawhub.NewClient(clawhub.GetRegistryURL(cfg), cfg.Token)
	if err := client.DeleteSkill(slug); err != nil {
		printError("Failed to delete skill: %v", err)
		os.Exit(1)
	}

	printSuccess("Deleted skill '%s'", slug)
}

func runUndelete(cmd *cobra.Command, args []string) {
	slug := args[0]

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

	// Confirm undeletion
	if !deleteYes && !confirm(fmt.Sprintf("Undelete skill '%s' from the registry?", slug)) {
		printInfo("Undeletion cancelled")
		return
	}

	// Undelete skill
	client := clawhub.NewClient(clawhub.GetRegistryURL(cfg), cfg.Token)
	if err := client.UndeleteSkill(slug); err != nil {
		printError("Failed to undelete skill: %v", err)
		os.Exit(1)
	}

	printSuccess("Undeleted skill '%s'", slug)
}
