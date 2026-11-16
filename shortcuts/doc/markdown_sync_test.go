// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareMarkdownForPushFrontmatterAndH1(t *testing.T) {
	t.Parallel()

	md := "---\ntitle: \"Quarterly Review\"\nauthor: someone\n---\n\n# Fallback Heading\n\nBody paragraph.\n"
	res, err := PrepareMarkdownForPush(md, PushMarkdownOptions{
		StripLeadingH1: true,
	})
	if err != nil {
		t.Fatalf("PrepareMarkdownForPush() error = %v", err)
	}
	if res.Title != "Quarterly Review" {
		t.Fatalf("Title = %q, want %q", res.Title, "Quarterly Review")
	}
	if !strings.HasPrefix(res.FullCreateContent, "<title>Quarterly Review</title>\n") {
		t.Fatalf("FullCreateContent missing frontmatter title prefix:\n%s", res.FullCreateContent)
	}
	if strings.Contains(res.Body, "author: someone") {
		t.Fatalf("frontmatter leaked into Body:\n%s", res.Body)
	}

	h1Only := "# Design Spec\n\nFirst section.\n"
	res2, err := PrepareMarkdownForPush(h1Only, PushMarkdownOptions{
		StripLeadingH1: true,
	})
	if err != nil {
		t.Fatalf("PrepareMarkdownForPush(h1Only) error = %v", err)
	}
	if res2.Title != "Design Spec" {
		t.Fatalf("Title = %q, want %q", res2.Title, "Design Spec")
	}
	if strings.Contains(res2.Body, "# Design Spec") {
		t.Fatalf("leading H1 was not stripped from Body:\n%s", res2.Body)
	}
	if strings.TrimSpace(res2.Body) != "First section." {
		t.Fatalf("Body = %q, want %q", res2.Body, "First section.")
	}

	keptH1, err := PrepareMarkdownForPush(h1Only, PushMarkdownOptions{
		KeepH1:         true,
		StripLeadingH1: true,
	})
	if err != nil {
		t.Fatalf("PrepareMarkdownForPush(KeepH1) error = %v", err)
	}
	if !strings.Contains(keptH1.Body, "# Design Spec") {
		t.Fatalf("KeepH1 should retain '# Design Spec' in Body:\n%s", keptH1.Body)
	}

	appended, err := PrepareMarkdownForPush(h1Only, PushMarkdownOptions{
		StripLeadingH1: false,
	})
	if err != nil {
		t.Fatalf("PrepareMarkdownForPush(append) error = %v", err)
	}
	if !strings.Contains(appended.Body, "# Design Spec") {
		t.Fatalf("append mode should retain '# Design Spec' in Body:\n%s", appended.Body)
	}
}

func TestPrepareMarkdownForPushProtectsMathCodeAndEscapesBareAngles(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"# Math & Safety",
		"",
		`Budget grew from \$500 to \$1000 and <USER_REQUEST> must survive.`,
		"",
		`Inline math $1 - \frac{x}{15}$ and comparison $a < b$ stay intact.`,
		"",
		"$$",
		"x < y && y > z",
		"$$",
		"",
		"Inline `<tag>` and link <https://example.com/a> and <br> in table:",
		"",
		"| A | B |",
		"|---|---|",
		"| l1<br />l2 | `<USER_REQUEST>` |",
		"",
		"```xml",
		"<USER_REQUEST>",
		"  ![ignored](./nonexistent.png)",
		"</USER_REQUEST>",
		"```",
	}, "\n")

	res, err := PrepareMarkdownForPush(input, PushMarkdownOptions{
		StripLeadingH1: true,
	})
	if err != nil {
		t.Fatalf("PrepareMarkdownForPush() error = %v", err)
	}

	for _, wantSub := range []string{
		`\$500 to \$1000`,
		`\<USER_REQUEST>`,
		`$1 - \frac{x}{15}$`,
		`$a < b$`,
		"$$\nx < y && y > z\n$$",
		"`<tag>`",
		"[https://example.com/a](https://example.com/a)",
		"<br/>",
		"```xml\n<USER_REQUEST>\n  ![ignored](./nonexistent.png)\n</USER_REQUEST>\n```",
	} {
		if !strings.Contains(res.Body, wantSub) {
			t.Errorf("Body missing expected substring %q\n--- got ---\n%s", wantSub, res.Body)
		}
	}
}

func TestPrepareMarkdownForPushResolvesArbitraryLocalImagesAndDeniesSecrets(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	docDir := filepath.Join(root, "notes")
	siblingDir := filepath.Join(root, "assets")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	relImgPath := filepath.Join(siblingDir, "rel.png")
	uriImgPath := filepath.Join(siblingDir, "uri.png")
	if err := os.WriteFile(relImgPath, []byte(localDocResourcePNG(t, 2, 3)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(uriImgPath, []byte(localDocResourcePNG(t, 4, 5)), 0o644); err != nil {
		t.Fatal(err)
	}

	md := strings.Join([]string{
		"# Images Outside CWD",
		"",
		"![relative](../assets/rel.png \"title1\")",
		"![abs](" + relImgPath + ")",
		"![fileuri](file://" + filepath.ToSlash(uriImgPath) + ")",
	}, "\n")

	res, err := PrepareMarkdownForPush(md, PushMarkdownOptions{
		SourcePath:     filepath.Join(docDir, "spec.md"),
		BaseDir:        docDir,
		StripLeadingH1: true,
	})
	if err != nil {
		t.Fatalf("PrepareMarkdownForPush(images) error = %v", err)
	}
	if len(res.Resources) != 3 {
		t.Fatalf("len(Resources) = %d, want 3", len(res.Resources))
	}
	if res.Resources[0].ImageWidth != 2 || res.Resources[0].ImageHeight != 3 || len(res.Resources[0].Content) == 0 {
		t.Fatalf("unexpected first image metadata: %+v", res.Resources[0])
	}
	if res.Resources[2].ImageWidth != 4 || res.Resources[2].ImageHeight != 5 {
		t.Fatalf("unexpected file:// image metadata: %+v", res.Resources[2])
	}
	for _, r := range res.Resources {
		if !strings.Contains(res.Body, `<img path="`+r.Marker+`"`) {
			t.Errorf("Body missing rewritten <img path=%q>: %s", r.Marker, res.Body)
		}
	}

	// Denylisted path (~/.ssh/id_rsa) must still be rejected.
	if _, err := PrepareMarkdownForPush("![secret](~/.ssh/id_rsa)\n", PushMarkdownOptions{BaseDir: docDir}); err == nil {
		t.Fatal("expected denylisted path ~/.ssh/id_rsa to fail, got nil")
	}
}

func TestNormalizePulledMarkdownRestoresTitleAndUnescapesSafeSpans(t *testing.T) {
	t.Parallel()

	pulled := strings.Join([]string{
		"<title>Architecture &amp; Sync</title>",
		"",
		`1\. First item \- with \*bold\* and \[checkbox\] and \<USER\_REQUEST\>`,
		"",
		`Cost is \$500 and path separator C:\\tmp stays intact\.`,
		"",
		`Math $a \* b$ and code ` + "`a\\.b`" + ` stay intact.`,
		"",
		"```bash",
		`echo "1\. keep inside fence"`,
		"```",
		"",
		"",
		"",
		"End paragraph.",
	}, "\n")

	got := NormalizePulledMarkdown("", pulled, false)
	for _, want := range []string{
		"# Architecture & Sync\n\n",
		"1. First item - with *bold* and [checkbox] and <USER_REQUEST>\n",
		`Cost is \$500 and path separator C:\\tmp stays intact.`,
		"Math $a \\* b$ and code `a\\.b` stay intact.\n",
		"```bash\necho \"1\\. keep inside fence\"\n```",
		"End paragraph.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("NormalizePulledMarkdown missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "<title>") {
		t.Errorf("NormalizePulledMarkdown left <title> tag behind:\n%s", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("NormalizePulledMarkdown failed to collapse extra blank lines:\n%s", got)
	}

	// Raw mode returns content untouched.
	if rawOut := NormalizePulledMarkdown("X", pulled, true); rawOut != pulled {
		t.Fatalf("raw mode modified content")
	}
}
