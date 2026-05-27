package cli

import "github.com/spf13/cobra"

// rootCmd represents the top-level CLI command
var rootCmd = &cobra.Command{
	Use:  "flbctl",
	Long: "flbctl helps user generate an opinionated Fluent-Bit YAML baseline and validate existing Fluent-Bit config file (only for Linux, tail input and stdout/loki output).",

	// Disable auto-generated completion command
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},

	SilenceUsage: true,
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

// init() is automatically executed by Go runtime when the cli package is initialized, this registers subcommands into the root command tree.
func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
}
