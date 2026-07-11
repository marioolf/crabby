// Package cli wires Crabby's commands together with Cobra.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/doctor"
	"github.com/marioolf/crabby/internal/initcmd"
	"github.com/marioolf/crabby/internal/project"
	"github.com/marioolf/crabby/internal/session"
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
		Use:           "crabby",
		Short:         "A lightweight workspace manager for Claude Code",
		Long:          "Crabby prepares projects for Claude Code and lets you manage multiple Claude sessions from one terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
	}
	// Make `crabby --version` match `crabby version`.
	root.SetVersionTemplate("Crabby v{{.Version}}\n")

	root.AddCommand(
		newInitCmd(),
		newPsCmd(),
		newAttachCmd(),
		newStartCmd(),
		newDoctorCmd(),
		newVersionCmd(),
	)
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Crabby version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Crabby v%s\n", version.String())
			return nil
		},
	}
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the current project for Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			res, err := initcmd.Init(cwd)
			if err != nil {
				return err
			}

			fmt.Printf("Initialized project %q\n", res.Project.Name)
			for _, f := range res.Created {
				fmt.Printf("  created %s\n", f)
			}
			if len(res.Created) == 0 {
				fmt.Println("  already initialized (registry updated)")
			}
			fmt.Printf("  session %s\n", res.Project.Session)
			fmt.Println("\nNext: run `crabby start` to launch Claude.")
			return nil
		},
	}
}

func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "List projects and attach to a Claude session",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			t := tmux.New(cfg.TmuxBinary)

			selected, err := tui.Run(t)
			if err != nil {
				return err
			}
			if selected == nil {
				return nil // user quit without choosing
			}
			return attach(cfg, *selected)
		},
	}
}

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach [project]",
		Short: "Attach to a project's Claude session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, err := resolveProject(args)
			if err != nil {
				return err
			}
			return attach(cfg, p)
		},
	}
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [project]",
		Short: "Create the session and launch Claude, then attach",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, err := resolveProject(args)
			if err != nil {
				return err
			}

			t := tmux.New(cfg.TmuxBinary)
			if !t.Available() {
				return fmt.Errorf("tmux not installed.\n\nInstall it with: sudo apt install tmux")
			}

			if !t.HasSession(p.Session) {
				if err := t.NewSession(p.Session, p.Path, cfg.ClaudeCommand); err != nil {
					return fmt.Errorf("could not create tmux session: %w", err)
				}
			}
			return t.Attach(p.Session)
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that the environment is ready for Crabby",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
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

// attach connects the user directly to a project's Claude session, or explains
// how to start it when the session is not running.
func attach(cfg config.Config, p project.Project) error {
	t := tmux.New(cfg.TmuxBinary)
	if !t.Available() {
		return fmt.Errorf("tmux not installed.\n\nInstall it with: sudo apt install tmux")
	}
	if !t.HasSession(p.Session) {
		return fmt.Errorf("session %q is not running (state: %s).\n\nRun:\n  crabby start %s",
			p.Session, session.Stopped, p.Name)
	}
	return t.Attach(p.Session)
}

// resolveProject finds the project either from an explicit name argument or
// from the current working directory.
func resolveProject(args []string) (project.Project, error) {
	if len(args) == 1 {
		p, err := project.Find(args[0])
		if err == project.ErrNotFound {
			return project.Project{}, fmt.Errorf("project %q is not registered.\n\nRun `crabby init` inside it first.", args[0])
		}
		return p, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return project.Project{}, err
	}
	p, err := project.FindByPath(cwd)
	if err == project.ErrNotFound {
		return project.Project{}, fmt.Errorf("this directory is not an initialized project.\n\nRun:\n  crabby init")
	}
	return p, err
}
