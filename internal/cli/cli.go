// Package cli wires Crabby's commands together with Cobra.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/marioolf/crabby/internal/config"
	"github.com/marioolf/crabby/internal/doctor"
	"github.com/marioolf/crabby/internal/importcmd"
	"github.com/marioolf/crabby/internal/initcmd"
	"github.com/marioolf/crabby/internal/insights"
	"github.com/marioolf/crabby/internal/pack"
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
		newTaskCmd(),
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

	// One collector for the whole home-screen loop, so its incremental
	// transcript cache survives returning from a session.
	coll := insights.New()
	for {
		res, err := tui.Run(t, cfg.DetachKey, coll)
		if err != nil {
			return err
		}
		switch res.Action {
		case tui.ActionQuit:
			return nil
		case tui.ActionOpen:
			if err := openTask(cfg, t, *res.Project, *res.Task); err != nil {
				pause(err)
			}
		case tui.ActionNewProject:
			if err := newProject(); err != nil {
				pause(err)
			}
		case tui.ActionNewTask:
			if err := newTaskInteractive(cfg, t, *res.Project); err != nil {
				pause(err)
			}
		}
	}
}

// newTaskInteractive is the dashboard's "t" key: ask for a task name, create it,
// and open it — so a parallel session starts without leaving Crabby.
func newTaskInteractive(cfg config.Config, t tmux.Client, p project.Project) error {
	fmt.Printf("New task in %q.\nTask name: ", p.Name)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	name := strings.TrimSpace(line)
	if name == "" {
		return nil
	}
	tk, err := createTask(p, name)
	if err != nil {
		return err
	}
	return openTask(cfg, t, p, tk)
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

// openTask launches Claude for a task if its session isn't running yet and
// attaches to it, blocking until the user leaves. All tasks in a workspace
// share the same project directory.
func openTask(cfg config.Config, t tmux.Client, p project.Project, task project.Task) error {
	if !t.HasSession(task.Session) {
		if _, err := lookClaude(cfg); err != nil {
			return err
		}
		if err := t.NewSession(task.Session, p.Path, cfg.ClaudeCommand); err != nil {
			return fmt.Errorf("could not start task %q in %q: %w", task.Name, p.Name, err)
		}
		// Show the workspace (and task, when named) in the status bar.
		_ = t.RenameWindow(task.Session, windowLabel(p, task))
	}
	// Best-effort: brand the session and enable the return key. A failure here
	// only means the status bar is plain, so don't block the attach.
	_ = t.Configure(cfg.DetachKey)
	return t.AttachChild(task.Session)
}

// windowLabel is what shows in the tmux status bar: just the workspace for the
// default task, workspace:task otherwise.
func windowLabel(p project.Project, task project.Task) string {
	if task.Name == project.DefaultTask {
		return p.Name
	}
	return p.Name + ":" + task.Name
}

// createTask validates a name, generates its session, and registers it in the
// workspace. It does not start the session — the caller opens it.
func createTask(p project.Project, name string) (project.Task, error) {
	if err := validateTaskName(name); err != nil {
		return project.Task{}, err
	}
	tk := project.Task{
		Name:    name,
		Session: session.TaskSession(p.Name, name),
		Created: time.Now(),
	}
	if err := project.AddTask(p.Name, tk); err != nil {
		return project.Task{}, fmt.Errorf("creating task %q: %w", name, err)
	}
	return tk, nil
}

// validateTaskName keeps task names safe for tmux session names and readable in
// the tree.
func validateTaskName(name string) error {
	if name == "" {
		return fmt.Errorf("a task name is required")
	}
	for _, r := range name {
		ok := r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !ok {
			return fmt.Errorf("task names may only contain letters, digits, '-' and '_' (got %q)", name)
		}
	}
	return nil
}

// chooseTask resolves which task to act on: an explicit name, the only task, or
// an interactive pick when several exist. The bool reports cancellation.
func chooseTask(p project.Project, taskArg string) (project.Task, bool, error) {
	if taskArg != "" {
		tk, ok := p.Task(taskArg)
		if !ok {
			return project.Task{}, false, fmt.Errorf("task %q not found in %q", taskArg, p.Name)
		}
		return tk, false, nil
	}
	if len(p.Tasks) == 1 {
		return p.Tasks[0], false, nil
	}
	chosen, err := tui.SelectTask(p.Name, p.Tasks)
	if err != nil {
		return project.Task{}, false, err
	}
	if chosen == nil {
		return project.Task{}, true, nil
	}
	return *chosen, false, nil
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

// newTaskCmd groups the task lifecycle: several independent Claude sessions may
// run inside one workspace, each its own task sharing the project directory.
func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks — parallel Claude sessions inside one workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newTaskCreateCmd(),
		newTaskListCmd(),
		newTaskDeleteCmd(),
		newTaskRestartCmd(),
	)
	return cmd
}

func newTaskCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a task in the current workspace and open it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			t := tmux.New(cfg.TmuxBinary)
			if !t.Available() {
				return errTmuxMissing(cfg.TmuxBinary)
			}
			p, err := resolveProject("")
			if err != nil {
				return err
			}
			tk, err := createTask(p, args[0])
			if err != nil {
				return err
			}
			return openTask(cfg, t, p, tk)
		},
	}
}

func newTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [workspace]",
		Short: "List the tasks in a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := resolveProject(arg(args, 0))
			if err != nil {
				return err
			}
			cfg, _ := config.Load()
			t := tmux.New(cfg.TmuxBinary)
			fmt.Println(p.Name)
			for _, tk := range p.Tasks {
				state := session.Classify(t.Available() && t.HasSession(tk.Session), false)
				fmt.Printf("  %-16s %s\n", tk.Name, state)
			}
			return nil
		},
	}
}

func newTaskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name> [workspace]",
		Short: "Delete a task (stops its session; the workspace stays)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			p, err := resolveProject(arg(args, 1))
			if err != nil {
				return err
			}
			tk, ok := p.Task(args[0])
			if !ok {
				return fmt.Errorf("task %q not found in %q", args[0], p.Name)
			}
			t := tmux.New(cfg.TmuxBinary)
			if t.Available() && t.HasSession(tk.Session) {
				_ = t.KillSession(tk.Session)
			}
			if err := project.RemoveTask(p.Name, tk.Name); err != nil {
				return err
			}
			fmt.Printf("Deleted task %q from %q.\n", tk.Name, p.Name)
			return nil
		},
	}
}

func newTaskRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <name> [workspace]",
		Short: "Stop a task's session and start it fresh",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			t := tmux.New(cfg.TmuxBinary)
			if !t.Available() {
				return errTmuxMissing(cfg.TmuxBinary)
			}
			p, err := resolveProject(arg(args, 1))
			if err != nil {
				return err
			}
			tk, ok := p.Task(args[0])
			if !ok {
				return fmt.Errorf("task %q not found in %q", args[0], p.Name)
			}
			if t.HasSession(tk.Session) {
				_ = t.KillSession(tk.Session)
			}
			return openTask(cfg, t, p, tk)
		},
	}
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
		Use:   "attach [workspace] [task]",
		Short: "Open a workspace's Claude session (asks which task when several exist)",
		Args:  cobra.MaximumNArgs(2),
		RunE:  attachRun,
	}
}

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start [workspace] [task]",
		Short: "Launch Claude for a workspace task and open it",
		Args:  cobra.MaximumNArgs(2),
		RunE:  attachRun,
	}
}

// attachRun backs both `attach` and `start`: resolve the workspace, pick a task
// (directly, or via a selector when several exist), and open it.
func attachRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	t := tmux.New(cfg.TmuxBinary)
	if !t.Available() {
		return errTmuxMissing(cfg.TmuxBinary)
	}
	p, err := resolveProject(arg(args, 0))
	if err != nil {
		return err
	}
	task, cancelled, err := chooseTask(p, arg(args, 1))
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}
	return openTask(cfg, t, p, task)
}

func newRmCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "rm [workspace]",
		Short: "Remove a workspace from Crabby (stops its tasks; does not delete files)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, err := resolveProject(arg(args, 0))
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

			// Stop every task's session first, then forget the workspace.
			t := tmux.New(cfg.TmuxBinary)
			if t.Available() {
				for _, tk := range p.Tasks {
					if t.HasSession(tk.Session) {
						_ = t.KillSession(tk.Session)
					}
				}
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

// resolveProject finds a workspace either from an explicit name or from the
// current working directory.
func resolveProject(name string) (project.Project, error) {
	if name != "" {
		p, err := project.Find(name)
		if err == project.ErrNotFound {
			return project.Project{}, fmt.Errorf("workspace %q is not registered.\n\nRun `crabby init` inside it first.", name)
		}
		return p, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return project.Project{}, err
	}
	p, err := project.FindByPath(cwd)
	if err == project.ErrNotFound {
		return project.Project{}, fmt.Errorf("this directory is not an initialized workspace.\n\nRun:\n  crabby init")
	}
	return p, err
}

// arg returns the nth argument or "" when absent.
func arg(args []string, n int) string {
	if n < len(args) {
		return args[n]
	}
	return ""
}
