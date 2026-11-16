// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package takeover

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/output"
)

// Status describes the current state of the optional `lark-cli -> larkx` symlink.
type Status struct {
	Active          bool   `json:"active"`
	Action          string `json:"action,omitempty"`
	Changed         bool   `json:"changed,omitempty"`
	BinDir          string `json:"bin_dir"`
	LinkPath        string `json:"link_path"`
	Target          string `json:"target,omitempty"`
	LinkDestination string `json:"link_destination,omitempty"`
	ResolvedLarkCLI string `json:"resolved_lark_cli,omitempty"`
	BinDirOnPATH    bool   `json:"bin_dir_on_path"`
	TakeoverFirst   bool   `json:"takeover_first_in_path"`
	Hint            string `json:"hint,omitempty"`
}

type options struct {
	factory *cmdutil.Factory
	binDir  string
	target  string
	force   bool
	asJSON  bool
}

// NewCmdTakeover creates the `takeover` root command for optionally managing
// a `<bin-dir>/lark-cli -> larkx` symlink without touching system installs.
func NewCmdTakeover(f *cmdutil.Factory) *cobra.Command {
	opts := &options{factory: f}

	cmd := &cobra.Command{
		Use:   "takeover",
		Short: "Manage optional lark-cli -> larkx symlink in user bin directory",
		Long: "Optionally shadow `lark-cli` with `larkx` via a symlink inside a user-owned bin\n" +
			"directory (default: ~/.local/bin) without modifying Homebrew/system binaries.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(opts)
		},
	}
	cmdutil.DisableAuthCheck(cmd)
	cmdutil.SetRisk(cmd, "read")

	cmd.PersistentFlags().StringVar(&opts.binDir, "bin-dir", defaultBinDir(), "directory containing the managed lark-cli symlink")
	cmd.PersistentFlags().StringVar(&opts.target, "target", "", "explicit path to the larkx binary (defaults to <bin-dir>/larkx or current executable)")
	cmd.PersistentFlags().BoolVar(&opts.force, "force", false, "on disable, remove a symlink even if it targets another binary (never removes regular files)")
	cmd.PersistentFlags().BoolVar(&opts.asJSON, "json", false, "output structured JSON")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show current lark-cli takeover status",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStatus(opts)
		},
	}
	cmdutil.DisableAuthCheck(statusCmd)
	cmdutil.SetRisk(statusCmd, "read")

	enableCmd := &cobra.Command{
		Use:   "enable",
		Short: "Create or update <bin-dir>/lark-cli symlink pointing to larkx",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runEnable(opts)
		},
	}
	cmdutil.DisableAuthCheck(enableCmd)
	cmdutil.SetRisk(enableCmd, "write")

	disableCmd := &cobra.Command{
		Use:   "disable",
		Short: "Remove managed <bin-dir>/lark-cli symlink pointing to larkx",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDisable(opts)
		},
	}
	cmdutil.DisableAuthCheck(disableCmd)
	cmdutil.SetRisk(disableCmd, "write")

	cmd.AddCommand(statusCmd, enableCmd, disableCmd)
	return cmd
}

func defaultBinDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".local", "bin")
	}
	return filepath.Join(".", ".local", "bin")
}

func expandPath(p string) (string, error) {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return "", nil
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if trimmed == "~" {
			return home, nil
		}
		trimmed = filepath.Join(home, trimmed[2:])
	}
	return filepath.Abs(trimmed)
}

func resolveBinDirAndTarget(opts *options) (binDir string, linkPath string, target string, err error) {
	rawDir := strings.TrimSpace(opts.binDir)
	if rawDir == "" {
		rawDir = defaultBinDir()
	}
	binDir, err = expandPath(rawDir)
	if err != nil {
		return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --bin-dir %q: %v", opts.binDir, err).WithParam("--bin-dir").WithCause(err)
	}
	linkPath = filepath.Join(binDir, "lark-cli")

	if strings.TrimSpace(opts.target) != "" {
		target, err = expandPath(opts.target)
		if err != nil {
			return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --target %q: %v", opts.target, err).WithParam("--target").WithCause(err)
		}
		return binDir, linkPath, target, nil
	}

	candidateInBin := filepath.Join(binDir, "larkx")
	if info, statErr := os.Stat(candidateInBin); statErr == nil && !info.IsDir() {
		return binDir, linkPath, candidateInBin, nil
	}
	if exe, exeErr := os.Executable(); exeErr == nil {
		if resolved, realErr := filepath.EvalSymlinks(exe); realErr == nil {
			exe = resolved
		}
		return binDir, linkPath, exe, nil
	}
	return binDir, linkPath, candidateInBin, nil
}

func computeStatus(opts *options) (Status, error) {
	binDir, linkPath, target, err := resolveBinDirAndTarget(opts)
	if err != nil {
		return Status{}, err
	}
	st := Status{
		BinDir:   binDir,
		LinkPath: linkPath,
		Target:   target,
	}

	if info, statErr := os.Lstat(linkPath); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if dest, readErr := os.Readlink(linkPath); readErr == nil {
				st.LinkDestination = dest
				if pointsToLarkx(linkPath, dest, target) {
					if _, targetErr := os.Stat(linkPath); targetErr == nil {
						st.Active = true
					}
				}
			}
		} else {
			st.LinkDestination = "(regular file)"
		}
	}

	if resolved, lookErr := exec.LookPath("lark-cli"); lookErr == nil {
		st.ResolvedLarkCLI = resolved
	}
	st.BinDirOnPATH, st.TakeoverFirst = checkPATHPriority(binDir)
	if !st.BinDirOnPATH {
		st.Hint = fmt.Sprintf("add %s to the front of PATH so `lark-cli` resolves before system installs", binDir)
	} else if !st.TakeoverFirst {
		st.Hint = fmt.Sprintf("ensure %s precedes other `lark-cli` directories (e.g. /opt/homebrew/bin) in PATH", binDir)
	}
	return st, nil
}

func runStatus(opts *options) error {
	st, err := computeStatus(opts)
	if err != nil {
		return err
	}
	st.Action = "status"
	return writeTakeoverOutput(opts, st)
}

func runEnable(opts *options) error {
	binDir, linkPath, target, err := resolveBinDirAndTarget(opts)
	if err != nil {
		return err
	}
	targetInfo, err := os.Stat(target)
	if err != nil || targetInfo.IsDir() {
		return errs.NewValidationError(errs.SubtypeFailedPrecondition, "target larkx binary %q does not exist or is a directory", target).WithParam("--target")
	}

	changed := false
	if info, lstatErr := os.Lstat(linkPath); lstatErr == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return errs.NewValidationError(
				errs.SubtypeFailedPrecondition,
				"refusing to overwrite non-symlink file %s; only symlinks are managed by takeover",
				linkPath,
			).WithParam("--bin-dir")
		}
		dest, readErr := os.Readlink(linkPath)
		if readErr == nil && filepath.Clean(resolveSymlinkTarget(linkPath, dest)) == filepath.Clean(target) {
			changed = false
		} else {
			if err := os.Remove(linkPath); err != nil {
				return errs.NewInternalError(errs.SubtypeUnknown, "remove existing symlink %s failed: %v", linkPath, err).WithCause(err)
			}
			changed = true
		}
	} else if !os.IsNotExist(lstatErr) {
		return errs.NewInternalError(errs.SubtypeUnknown, "inspect %s failed: %v", linkPath, lstatErr).WithCause(lstatErr)
	} else {
		changed = true
	}

	if changed {
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return errs.NewInternalError(errs.SubtypeUnknown, "create bin dir %s failed: %v", binDir, err).WithCause(err)
		}
		if err := os.Symlink(target, linkPath); err != nil {
			return errs.NewInternalError(errs.SubtypeUnknown, "create symlink %s -> %s failed: %v", linkPath, target, err).WithCause(err)
		}
	}

	st, err := computeStatus(opts)
	if err != nil {
		return err
	}
	st.Action = "enable"
	st.Changed = changed
	return writeTakeoverOutput(opts, st)
}

func runDisable(opts *options) error {
	_, linkPath, target, err := resolveBinDirAndTarget(opts)
	if err != nil {
		return err
	}

	changed := false
	info, lstatErr := os.Lstat(linkPath)
	switch {
	case os.IsNotExist(lstatErr):
		changed = false
	case lstatErr != nil:
		return errs.NewInternalError(errs.SubtypeUnknown, "inspect %s failed: %v", linkPath, lstatErr).WithCause(lstatErr)
	case info.Mode()&os.ModeSymlink == 0:
		return errs.NewValidationError(
			errs.SubtypeFailedPrecondition,
			"refusing to remove non-symlink file %s; only symlinks are managed by takeover",
			linkPath,
		).WithParam("--bin-dir")
	default:
		dest, readErr := os.Readlink(linkPath)
		if readErr != nil {
			return errs.NewInternalError(errs.SubtypeUnknown, "read symlink %s failed: %v", linkPath, readErr).WithCause(readErr)
		}
		if !opts.force && !pointsToLarkx(linkPath, dest, target) {
			return errs.NewValidationError(
				errs.SubtypeFailedPrecondition,
				"refusing to remove symlink %s -> %s (does not target larkx; pass --force to remove)",
				linkPath, dest,
			).WithParam("--force")
		}
		if err := os.Remove(linkPath); err != nil {
			return errs.NewInternalError(errs.SubtypeUnknown, "remove symlink %s failed: %v", linkPath, err).WithCause(err)
		}
		changed = true
	}

	st, err := computeStatus(opts)
	if err != nil {
		return err
	}
	st.Action = "disable"
	st.Changed = changed
	return writeTakeoverOutput(opts, st)
}

func resolveSymlinkTarget(linkPath, dest string) string {
	if filepath.IsAbs(dest) {
		return filepath.Clean(dest)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(linkPath), dest))
}

func pointsToLarkx(linkPath, dest, target string) bool {
	if filepath.Base(dest) == "larkx" {
		return true
	}
	if target != "" && filepath.Clean(resolveSymlinkTarget(linkPath, dest)) == filepath.Clean(target) {
		return true
	}
	return false
}

func checkPATHPriority(binDir string) (onPATH bool, precedesOtherLarkCLI bool) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return false, false
	}
	cleanBin := filepath.Clean(binDir)
	precedesOtherLarkCLI = true
	sawOtherBeforeBin := false

	for _, entry := range filepath.SplitList(pathEnv) {
		if entry == "" {
			continue
		}
		cleanEntry := filepath.Clean(entry)
		if cleanEntry == cleanBin {
			onPATH = true
			if sawOtherBeforeBin {
				precedesOtherLarkCLI = false
			}
			break
		}
		if cleanEntry == filepath.Clean("/opt/homebrew/bin") {
			sawOtherBeforeBin = true
			continue
		}
		if info, err := os.Stat(filepath.Join(cleanEntry, "lark-cli")); err == nil && !info.IsDir() {
			sawOtherBeforeBin = true
		}
	}
	if !onPATH {
		return false, false
	}
	return true, precedesOtherLarkCLI
}

func writeTakeoverOutput(opts *options, st Status) error {
	var out io.Writer = os.Stdout
	if opts.factory != nil && opts.factory.IOStreams != nil && opts.factory.IOStreams.Out != nil {
		out = opts.factory.IOStreams.Out
	}
	if opts.asJSON {
		output.PrintJson(out, st)
		return nil
	}

	state := "disabled"
	if st.Active {
		state = "enabled"
	}
	fmt.Fprintf(out, "takeover: %s\n", state)
	fmt.Fprintf(out, "link:     %s", st.LinkPath)
	if st.LinkDestination != "" {
		fmt.Fprintf(out, " -> %s", st.LinkDestination)
	}
	fmt.Fprintln(out)
	if st.Target != "" {
		fmt.Fprintf(out, "target:   %s\n", st.Target)
	}
	if st.ResolvedLarkCLI != "" {
		fmt.Fprintf(out, "PATH:     `lark-cli` resolves to %s\n", st.ResolvedLarkCLI)
	}
	if st.Hint != "" {
		fmt.Fprintf(out, "hint:     %s\n", st.Hint)
	}
	return nil
}
