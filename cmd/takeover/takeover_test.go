// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package takeover

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
)

func runTakeoverTestCmd(t *testing.T, args []string) (Status, error) {
	t.Helper()
	var stdout bytes.Buffer
	f := &cmdutil.Factory{
		IOStreams: &cmdutil.IOStreams{Out: &stdout, ErrOut: &stdout},
	}
	cmd := NewCmdTakeover(f)
	cmd.SetArgs(append([]string{"--json"}, args...))
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err != nil {
		return Status{}, err
	}
	var st Status
	if err := json.Unmarshal(stdout.Bytes(), &st); err != nil {
		t.Fatalf("decode takeover json %q: %v", stdout.String(), err)
	}
	return st, nil
}

func TestTakeoverEnableStatusDisableLifecycle(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	target := filepath.Join(binDir, "larkx")
	if err := os.WriteFile(target, []byte("#!/bin/sh\necho larkx\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "status"})
	if err != nil {
		t.Fatalf("status() error = %v", err)
	}
	if st.Active {
		t.Fatalf("initial status should be inactive: %+v", st)
	}

	st, err = runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "enable"})
	if err != nil {
		t.Fatalf("enable() error = %v", err)
	}
	if !st.Active || !st.Changed {
		t.Fatalf("enable() = %+v, want active & changed", st)
	}
	dest, err := os.Readlink(filepath.Join(binDir, "lark-cli"))
	if err != nil || dest != target {
		t.Fatalf("Readlink = %q (%v), want %q", dest, err, target)
	}

	// Idempotent re-enable.
	st, err = runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "enable"})
	if err != nil {
		t.Fatalf("second enable() error = %v", err)
	}
	if !st.Active || st.Changed {
		t.Fatalf("second enable() = %+v, want active & unchanged", st)
	}

	st, err = runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "disable"})
	if err != nil {
		t.Fatalf("disable() error = %v", err)
	}
	if st.Active || !st.Changed {
		t.Fatalf("disable() = %+v, want inactive & changed", st)
	}

	// Idempotent disable.
	st, err = runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "disable"})
	if err != nil {
		t.Fatalf("second disable() error = %v", err)
	}
	if st.Active || st.Changed {
		t.Fatalf("second disable() = %+v, want inactive & unchanged", st)
	}
}

func TestTakeoverRefusesToClobberRegularFile(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	target := filepath.Join(binDir, "larkx")
	if err := os.WriteFile(target, []byte("larkx"), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(binDir, "lark-cli")
	if err := os.WriteFile(existing, []byte("homebrew binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "enable"}); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("enable() err = %v, want non-symlink refusal", err)
	}
	if _, err := runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "disable", "--force"}); err == nil || !strings.Contains(err.Error(), "non-symlink") {
		t.Fatalf("disable --force err = %v, want non-symlink refusal", err)
	}
}

func TestTakeoverDisableRefusesForeignSymlinkWithoutForce(t *testing.T) {
	t.Parallel()

	binDir := t.TempDir()
	other := filepath.Join(binDir, "other-tool")
	if err := os.WriteFile(other, []byte("other"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(binDir, "lark-cli")
	if err := os.Symlink(other, linkPath); err != nil {
		t.Fatal(err)
	}

	if _, err := runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "disable"}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("disable() err = %v, want --force hint", err)
	}

	st, err := runTakeoverTestCmd(t, []string{"--bin-dir", binDir, "disable", "--force"})
	if err != nil {
		t.Fatalf("disable --force error = %v", err)
	}
	if st.Active || !st.Changed {
		t.Fatalf("disable --force = %+v, want changed", st)
	}
}
