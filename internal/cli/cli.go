// Package cli wires Crabby's commands together with Cobra.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/doctor"
	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/initcmd"
	"github.com/marioolf/crabby/internal/pack"
	"github.com/marioolf/crabby/internal/project"
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
			"Run it with no arguments to open the home screen, pick a project, and jump straight into Claude.\n\n" +
			version.Repo,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.String(),
		Args:          cobra.NoArgs,
		// `crabby` with no subcommand opens the home screen.
		RunE: func(cmd *cobra.Command, args []string) error {
			return home()
		},
	}
	root.SetVersionTemplate("Crabby v{{.Version}}\n")
	// Keep the CLI minimal — hide Cobra's auto-generated `completion` command.
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newInitCmd(),
		newImportCmd(),
		newPsCmd(),
		newAttachCmd(),
		newStartCmd(),
		newRmCmd(),
		newDoctorCmd(),
		newVersionCmd(),
	)
	return root
}

// home is Crabby's main loop: show the project list, open the chosen session,
// and — when the user leaves it — return to the list. The user stays inside
// Crabby the whole time.
func home() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	t := tmux.New(cfg.TmuxBinary)
	if !t.Available() {
		return errTmuxMissing(cfg.TmuxBinary)
	}

	for {
		res, err := tui.Run(t, cfg.DetachKey)
		if err != nil {
			return err
		}
		switch res.Action {
		case tui.ActionQuit:
			return nil
		case tui.ActionOpen:
			if err := openSession(cfg, t, *res.Project); err != nil {
				pause(err)
			}
		case tui.ActionNewProject:
			if err := newProject(); err != nil {
				pause(err)
			}
		}
	}
}

// pause shows a problem and waits, so it isn't lost when the home screen
// redraws over it.
func pause(err error) {
	fmt.Fprintln(os.Stderr, "\ncrabby:", err)
	fmt.Fprint(os.Stderr, "\nPress Enter to return to Crabby...")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

// newProject initializes the current working directory as a project, choosing
// a pack when more than one is available.
func newProject() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	p, cancelled, err := choosePack()
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}
	res, err := initcmd.Init(cwd, p)
	if err != nil {
		return err
	}
	printInitSummary(res)
	fmt.Print("\nPress Enter to return to Crabby...")
	bufio.NewReader(os.Stdin).ReadString('\n')
	return nil
}

// choosePack decides which pack `init` should use: none (built-in default),
// the only one available, or one picked from a selector. The bool reports
// whether the user cancelled the selection.
func choosePack() (p *pack.Pack, cancelled bool, err error) {
	packs, err := pack.List()
	if err != nil {
		return nil, false, err
	}
	switch len(packs) {
	case 0:
		return nil, false, nil
	case 1:
		return &packs[0], false, nil
	default:
		chosen, err := tui.SelectPack(packs)
		if err != nil {
			return nil, false, err
		}
		return chosen, chosen == nil, nil
	}
}

func printInitSummary(res initcmd.Result) {
	if res.PackID != "" {
		fmt.Printf("Initialized %q using pack %q\n", res.Project.Name, res.PackID)
	} else {
		fmt.Printf("Initialized %q\n", res.Project.Name)
	}
	for _, f := range res.Created {
		fmt.Printf("  + %s\n", f)
	}
	for _, f := range res.Skipped {
		fmt.Printf("  ! %s (already exists — kept)\n", f)
	}
	if res.PackID == "" {
		fmt.Printf("\nTip: drop reusable project packs in %s to customize new projects.\n", pack.DisplayDir())
		fmt.Println("     Copy an example: cp -r examples/<name> " + pack.DisplayDir() + "/<name>")
	}
}

// openSession launches Claude for a project if needed and attaches to it,
// blocking until the user leaves. Selecting a project is all it takes — a
// stopped project is started automatically.
func openSession(cfg config.Config, t tmux.Client, p project.Project) error {
	if !t.HasSession(p.Session) {
		if _, err := lookClaude(cfg); err != nil {
			return err
		}
		if err := t.NewSession(p.Session, p.Path, cfg.ClaudeCommand); err != nil {
			return fmt.Errorf("could not start a session for %q: %w", p.Name, err)
		}
		// Show the project name in the status bar, not the command.
		_ = t.RenameWindow(p.Session, p.Name)
	}
	// Best-effort: brand the session and enable the return key. A failure here
	// only means the status bar is plain, so don't block the attach.
	_ = t.Configure(cfg.DetachKey)
	return t.AttachChild(p.Session)
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

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a project for Claude Code (current directory by default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				dir = args[0]
			}
			p, cancelled, err := choosePack()
			if err != nil {
				return err
			}
			if cancelled {
				return nil
			}
			res, err := initcmd.Init(dir, p)
			if err != nil {
				return err
			}
			printInitSummary(res)
			fmt.Println("\nNext: run `crabby` and press Enter on this project to open it.")
			return nil
		},
	}
}

// newImportCmd adopts existing Claude Code repositories in bulk: it scans a
// directory tree for git repos that already have a CLAUDE.md and registers the
// ones the user picks. No files are changed beyond Crabby's own metadata.
func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import [path]",
		Short: "Import existing Claude Code repositories into Crabby",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			fmt.Printf("Scanning %s ...\n", root)
			found, err := importcmd.Discover(root)
			if err != nil {
				return err
			}
			if len(found) == 0 {
				fmt.Printf("No Claude workspaces found under %s.\n", root)
				fmt.Println("A workspace is any folder containing a CLAUDE.md.")
				return nil
			}

			chosen, cancelled, err := tui.SelectImports(found)
			if err != nil {
				return err
			}
			if cancelled || len(chosen) == 0 {
				fmt.Println("Nothing imported.")
				return nil
			}

			for _, w := range chosen {
				// Init with no pack registers the project and writes Crabby's
				// metadata without touching the repo's existing CLAUDE.md.
				res, err := initcmd.Init(w.Path, nil)
				if err != nil {
					fmt.Printf("  ✗ %s (%v)\n", w.Name, err)
					continue
				}
				fmt.Printf("  + %s\n", res.Project.Name)
			}
			fmt.Printf("\nImported %s. Run `crabby` to see them.\n", pluralWord(len(chosen), "project"))
			return nil
		},
	}
}

// pluralWord formats a count with its noun for import summaries.
func pluralWord(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// newPsCmd keeps `crabby ps` working as an alias for the home screen.
func newPsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ps",
		Short: "Open the Crabby home screen (alias for `crabby`)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return home()
		},
	}
}

func newAttachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach [project]",
		Short: "Open a project's Claude session directly",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			t := tmux.New(cfg.TmuxBinary)
			if !t.Available() {
				return errTmuxMissing(cfg.TmuxBinary)
			}
			p, err := resolveProject(args)
			if err != nil {
				return err
			}
			return openSession(cfg, t, p)
		},
	}
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [project]",
		Short: "Launch Claude for a project and open it",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			t := tmux.New(cfg.TmuxBinary)
			if !t.Available() {
				return errTmuxMissing(cfg.TmuxBinary)
			}
			p, err := resolveProject(args)
			if err != nil {
				return err
			}
			return openSession(cfg, t, p)
		},
	}
}

func newRmCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rm [project]",
		Short: "Remove a project from Crabby (does not delete files)",
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

			if !yes {
				fmt.Printf("Remove %q from Crabby? Your files are kept. (y/N): ", p.Name)
				line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if a := strings.TrimSpace(strings.ToLower(line)); a != "y" && a != "yes" {
					fmt.Println("Cancelled.")
					return nil
				}
			}

			// Stop the session first if it's running, then forget the project.
			t := tmux.New(cfg.TmuxBinary)
			if t.Available() && t.HasSession(p.Session) {
				_ = t.KillSession(p.Session)
			}
			if err := project.Remove(p.Name); err != nil {
				return err
			}
			fmt.Printf("Removed %q. Files left untouched — run `crabby init` there to re-add it.\n", p.Name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
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

func lookClaude(cfg config.Config) (string, error) {
	path, err := exec.LookPath(cfg.ClaudeCommand)
	if err != nil {
		return "", fmt.Errorf("Claude Code not found (looked for %q).\n\nInstall it from https://claude.com/claude-code, or set claude_command in ~/.config/crabby/config.yaml", cfg.ClaudeCommand)
	}
	return path, nil
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
