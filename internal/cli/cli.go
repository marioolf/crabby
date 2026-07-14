// Package cli wires Crabby's commands together with Cobra.
//
// From v0.8 Crabby is TUI-first: `crabby` opens the application and every
// day-to-day action lives inside it. The CLI keeps only the two commands that
// make sense outside the interface — `version` and `doctor`.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/doctor"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/tmux"
	"github.com/marioolf/crabby/internal/tui"
	"github.com/marioolf/crabby/internal/version"
)

// Execute runs the root command. It returns a process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "crabby",
		Short: "A workspace manager for Claude Code",
		Long: tui.Banner() + "\n\n" +
			"A workspace manager for Claude Code.\n" +
			"Run it with no arguments to open the app: create workspaces and tasks,\n" +
			"jump into Claude, and manage packs — all without leaving the terminal.\n\n" +
			version.Repo,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		Args:          cobra.NoArgs,
		// `crabby` with no subcommand opens the application.
		RunE: func(cmd *cobra.Command, args []string) error {
			return home()
		},
	}
	root.SetVersionTemplate("Crabby v{{.Version}}\n")
	// Keep the CLI minimal — hide Cobra's auto-generated `completion` command.
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newVersionCmd(),
		newDoctorCmd(),
	)
	return root
}

// home launches the persistent TUI. The user stays inside it for the whole
// session; it returns only when the user quits.
func home() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	t := tmux.New(cfg.TmuxBinary)
	if !t.Available() {
		return errTmuxMissing(cfg.TmuxBinary)
	}
	// One collector for the whole session so its incremental transcript cache
	// survives returning from a Claude session.
	return tui.Run(cfg, t, insights.New())
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Crabby version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("%s v%s\n", version.App, version.String())
			fmt.Printf("by %s · %s\n", version.Author, version.Repo)
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that the environment is ready for Crabby",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Printf("%s v%s  ·  %s\n\n", version.App, version.String(), version.Repo)
			allOK := true
			for _, c := range doctor.Run(cfg) {
				mark := "✓"
				if !c.OK {
					mark = "✗"
					allOK = false
				}
				line := fmt.Sprintf("%s %s", mark, c.Name)
				if c.Note != "" {
					line += fmt.Sprintf("  (%s)", c.Note)
				}
				fmt.Println(line)
			}
			if !allOK {
				return fmt.Errorf("some checks failed")
			}
			return nil
		},
	}
}

func errTmuxMissing(binary string) error {
	return fmt.Errorf("Crabby needs tmux, which was not found (looked for %q).\n\nInstall it with:  sudo apt install tmux", binary)
}
