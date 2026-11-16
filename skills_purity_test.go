// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var purityMarkdownLinkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

func TestSkillsPurity(t *testing.T) {
	banned := []string{
		"lark-doc-create-workflow",
		"init-draft",
		"presentation-decision",
		"genre_contract",
		"体裁契约",
		"读者本位",
		"简单任务不是跳过的理由",
		"creative-design",
		"豆包设计原则",
		"docs +script",
		"lark-whiteboard-workflow",
		"lark-drive-workflow",
		"lark-sheets-visual-standards",
		"lark-im-card-style",
	}

	targetExts := map[string]bool{
		".md":   true,
		".xml":  true,
		".json": true,
		".py":   true,
		".js":   true,
	}

	for _, rootDir := range []string{"skills", "affordance"} {
		err := fs.WalkDir(embeddedContentFS, rootDir, func(fsPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || path.Ext(fsPath) != ".md" {
				return nil
			}
			data, err := fs.ReadFile(embeddedContentFS, fsPath)
			if err != nil {
				return err
			}
			text := string(data)
			for _, token := range banned {
				if strings.Contains(text, token) {
					t.Errorf("%s contains banned workflow/style reference %q", fsPath, token)
				}
			}

			scrubbed := stripCodeForLinkCheck(text)
			diskDir := filepath.Dir(filepath.FromSlash(fsPath))
			for _, match := range purityMarkdownLinkRE.FindAllStringSubmatch(scrubbed, -1) {
				target := match[1]
				if strings.HasPrefix(target, "http://") ||
					strings.HasPrefix(target, "https://") ||
					strings.HasPrefix(target, "mailto:") ||
					strings.HasPrefix(target, "#") {
					continue
				}
				if idx := strings.IndexByte(target, '#'); idx >= 0 {
					target = target[:idx]
				}
				if target == "" {
					continue
				}
				ext := strings.ToLower(path.Ext(target))
				if !targetExts[ext] {
					continue
				}
				resolved := filepath.Clean(filepath.Join(diskDir, filepath.FromSlash(target)))
				if _, statErr := os.Stat(resolved); statErr != nil {
					t.Errorf("%s has dead link %q (resolved: %s): %v", fsPath, match[1], resolved, statErr)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WalkDir(%s) error = %v", rootDir, err)
		}
	}
}

func stripCodeForLinkCheck(s string) string {
	lines := strings.Split(s, "\n")
	var out strings.Builder
	var fenceChar byte
	var fenceLen int
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if fenceChar == 0 {
			if ch, n := leadingFenceRun(trimmed); n >= 3 {
				fenceChar, fenceLen = ch, n
				out.WriteByte('\n')
				continue
			}
			out.WriteString(stripInlineBackticks(line))
			out.WriteByte('\n')
			continue
		}
		if ch, n := leadingFenceRun(trimmed); ch == fenceChar && n >= fenceLen && strings.TrimSpace(trimmed[n:]) == "" {
			fenceChar, fenceLen = 0, 0
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func leadingFenceRun(s string) (byte, int) {
	if s == "" || (s[0] != '`' && s[0] != '~') {
		return 0, 0
	}
	ch := s[0]
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return ch, n
}

func stripInlineBackticks(line string) string {
	var out strings.Builder
	for i := 0; i < len(line); {
		if line[i] != '`' {
			out.WriteByte(line[i])
			i++
			continue
		}
		run := 0
		for i+run < len(line) && line[i+run] == '`' {
			run++
		}
		j := i + run
		closed := false
		for j < len(line) {
			if line[j] != '`' {
				j++
				continue
			}
			endRun := 0
			for j+endRun < len(line) && line[j+endRun] == '`' {
				endRun++
			}
			if endRun == run {
				i = j + endRun
				closed = true
				break
			}
			j += endRun
		}
		if !closed {
			out.WriteString(line[i : i+run])
			i += run
		}
	}
	return out.String()
}
