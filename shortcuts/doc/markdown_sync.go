// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/imageconfig"
	"github.com/larksuite/cli/internal/vfs"
	"github.com/larksuite/cli/internal/vfs/localfileio"
	"github.com/larksuite/cli/shortcuts/common"
)

// PushMarkdownOptions configures Markdown normalization and local image
// extraction before pushing to Lark Docs.
type PushMarkdownOptions struct {
	SourcePath      string
	BaseDir         string
	TitleOverride   string
	KeepH1          bool
	KeepFrontmatter bool
	AllowXMLTags    bool
	StripLeadingH1  bool
	FallbackToStem  bool
}

// PreparedMarkdownPush holds the normalized document title, compiled body,
// full create content (with <title> prefix), and local images queued for upload.
type PreparedMarkdownPush struct {
	Title             string
	Body              string
	FullCreateContent string
	Resources         []localDocResource
}

// PrepareMarkdownForPush compiles standard Markdown into a docs_ai-ready
// markdown payload while preserving code/math/currency spans, escaping bare '<',
// extracting title metadata, and resolving arbitrary local image paths.
func PrepareMarkdownForPush(raw string, opts PushMarkdownOptions) (*PreparedMarkdownPush, error) {
	normalized := normalizeLineEndings(strings.TrimPrefix(raw, "\xef\xbb\xbf"))

	var fmTitle string
	body := normalized
	if fm, rest, found := splitYAMLFrontmatter(normalized); found {
		fmTitle = extractFrontmatterTitle(fm)
		if !opts.KeepFrontmatter {
			body = strings.TrimLeft(rest, "\n")
		}
	}

	h1Title, bodyAfterH1 := extractLeadingH1(body)
	if h1Title != "" && opts.StripLeadingH1 && !opts.KeepH1 && !opts.KeepFrontmatter {
		override := strings.TrimSpace(opts.TitleOverride)
		if override == "" || override == h1Title {
			body = bodyAfterH1
		}
	}

	title := strings.TrimSpace(opts.TitleOverride)
	if title == "" {
		title = fmTitle
	}
	if title == "" {
		title = h1Title
	}
	if title == "" && opts.FallbackToStem && opts.SourcePath != "" && opts.SourcePath != "-" {
		base := filepath.Base(opts.SourcePath)
		stem := strings.TrimSuffix(base, filepath.Ext(base))
		title = strings.TrimSpace(stem)
	}

	compiledBody, resources, err := compileMarkdownBody(body, opts)
	if err != nil {
		return nil, err
	}

	fullCreate := compiledBody
	if title != "" {
		titleTag := "<title>" + escapeDocTitleText(title) + "</title>"
		if strings.TrimSpace(compiledBody) == "" {
			fullCreate = titleTag
		} else {
			fullCreate = titleTag + "\n" + compiledBody
		}
	}

	return &PreparedMarkdownPush{
		Title:             title,
		Body:              compiledBody,
		FullCreateContent: fullCreate,
		Resources:         resources,
	}, nil
}

// NormalizePulledMarkdown cleans docs_ai exported markdown for local editing:
// it promotes <title> to a leading # H1 (when absent), removes redundant
// backslash escapes outside code/math spans while preserving \\ and \$, and
// normalizes spacing.
func NormalizePulledMarkdown(docTitle, rawContent string, raw bool) string {
	if raw {
		return rawContent
	}
	content := normalizeLineEndings(strings.TrimPrefix(rawContent, "\xef\xbb\xbf"))
	extractedTitle, body := stripLeadingDocTitleTag(content)
	if strings.TrimSpace(docTitle) == "" {
		docTitle = extractedTitle
	}
	docTitle = strings.TrimSpace(docTitle)

	chunks := splitMarkdownFenceChunks(body)
	var out strings.Builder
	prevEndedWithNewline := false
	for _, chunk := range chunks {
		if chunk.isFence {
			out.WriteString(chunk.text)
			prevEndedWithNewline = strings.HasSuffix(chunk.text, "\n")
			continue
		}
		unescaped := collapseExcessBlankLines(unescapePulledMarkdownProse(chunk.text))
		if prevEndedWithNewline && strings.HasPrefix(unescaped, "\n\n") {
			unescaped = "\n" + strings.TrimLeft(unescaped, "\n")
		}
		out.WriteString(unescaped)
		prevEndedWithNewline = false
	}

	cleaned := strings.Trim(out.String(), "\n")
	if docTitle != "" {
		if cleaned == "" {
			return "# " + docTitle + "\n"
		}
		if !startsWithMatchingH1(cleaned, docTitle) {
			cleaned = "# " + docTitle + "\n\n" + cleaned
		}
	}
	if cleaned == "" {
		return ""
	}
	return cleaned + "\n"
}

func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

func splitYAMLFrontmatter(s string) (frontmatter string, rest string, ok bool) {
	lines := strings.SplitAfter(s, "\n")
	if len(lines) < 2 {
		return "", s, false
	}
	first := strings.TrimRight(lines[0], "\n")
	if strings.TrimSpace(first) != "---" {
		return "", s, false
	}
	pos := len(lines[0])
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(strings.TrimRight(lines[i], "\n"))
		if trimmed == "---" || trimmed == "..." {
			return s[len(lines[0]):pos], s[pos+len(lines[i]):], true
		}
		pos += len(lines[i])
	}
	return "", s, false
}

func extractFrontmatterTitle(fm string) string {
	for _, line := range strings.Split(fm, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		if !strings.EqualFold(key, "title") {
			continue
		}
		val := strings.TrimSpace(line[colon+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				quote := val[0]
				val = val[1 : len(val)-1]
				if quote == '"' {
					val = strings.ReplaceAll(val, `\"`, `"`)
					val = strings.ReplaceAll(val, `\\`, `\`)
				} else {
					val = strings.ReplaceAll(val, `''`, `'`)
				}
			}
		}
		return strings.TrimSpace(val)
	}
	return ""
}

func extractLeadingH1(body string) (title string, strippedBody string) {
	lines := strings.SplitAfter(body, "\n")
	firstIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstIdx = i
			break
		}
	}
	if firstIdx < 0 {
		return "", body
	}
	firstLine := strings.TrimRight(lines[firstIdx], "\n")
	if _, _, isFence := markdownFence(firstLine); isFence {
		return "", body
	}
	if !strings.HasPrefix(firstLine, "#") {
		return "", body
	}
	if len(firstLine) < 2 || (firstLine[1] != ' ' && firstLine[1] != '\t') {
		return "", body
	}
	heading := strings.TrimSpace(firstLine[1:])
	heading = trimATXClosingHashes(heading)
	heading = strings.TrimSpace(unescapeMarkdownText(heading))
	if heading == "" {
		return "", body
	}

	nextIdx := firstIdx + 1
	if nextIdx < len(lines) && strings.TrimSpace(lines[nextIdx]) == "" {
		nextIdx++
	}
	var out strings.Builder
	for i := nextIdx; i < len(lines); i++ {
		out.WriteString(lines[i])
	}
	return heading, out.String()
}

func trimATXClosingHashes(s string) string {
	trimmed := strings.TrimRight(s, " \t")
	noHash := strings.TrimRight(trimmed, "#")
	if len(noHash) < len(trimmed) && (noHash == "" || noHash[len(noHash)-1] == ' ' || noHash[len(noHash)-1] == '\t') {
		return strings.TrimRight(noHash, " \t")
	}
	return trimmed
}

type markdownChunk struct {
	isFence bool
	text    string
}

func splitMarkdownFenceChunks(s string) []markdownChunk {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	var chunks []markdownChunk
	var buf strings.Builder
	var fenceChar byte
	var fenceRun int

	flush := func(isFence bool) {
		if buf.Len() == 0 {
			return
		}
		chunks = append(chunks, markdownChunk{isFence: isFence, text: buf.String()})
		buf.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\n")
		if fenceChar == 0 {
			if ch, run, ok := markdownFence(trimmed); ok {
				flush(false)
				fenceChar, fenceRun = ch, run
				buf.WriteString(line)
				continue
			}
			buf.WriteString(line)
			continue
		}
		buf.WriteString(line)
		if markdownFenceCloses(trimmed, fenceChar, fenceRun) {
			flush(true)
			fenceChar, fenceRun = 0, 0
		}
	}
	flush(fenceChar != 0)
	return chunks
}

func compileMarkdownBody(body string, opts PushMarkdownOptions) (string, []localDocResource, error) {
	chunks := splitMarkdownFenceChunks(body)
	var out strings.Builder
	var resources []localDocResource

	for _, chunk := range chunks {
		if chunk.isFence {
			out.WriteString(chunk.text)
			continue
		}
		seg := chunk.text
		for i := 0; i < len(seg); {
			if seg[i] == '`' {
				run := countByteRun(seg, i, '`')
				if end := findMatchingBacktickRun(seg, i+run, run); end > 0 {
					out.WriteString(seg[i:end])
					i = end
					continue
				}
				out.WriteString(seg[i : i+run])
				i += run
				continue
			}

			if seg[i] == '\\' {
				if i+1 < len(seg) {
					out.WriteString(seg[i : i+2])
					i += 2
				} else {
					out.WriteByte('\\')
					i++
				}
				continue
			}

			if seg[i] == '$' {
				if end, ok := findMathSpanEnd(seg, i); ok {
					out.WriteString(seg[i:end])
					i = end
					continue
				}
				out.WriteByte('$')
				i++
				continue
			}

			if seg[i] == '!' && i+1 < len(seg) && seg[i+1] == '[' {
				if img, ok := parseMarkdownImageAt(seg, i); ok && img.ReferenceLabel == "" {
					replacement, resource, added, err := compileMarkdownImage(seg[i:img.End], img, opts, len(resources)+1)
					if err != nil {
						return "", nil, err
					}
					out.WriteString(replacement)
					if added {
						resources = append(resources, resource)
					}
					i = img.End
					continue
				}
			}

			if seg[i] == '<' {
				if end, normalizedBR, ok := matchHTMLBreakTag(seg, i); ok {
					out.WriteString(normalizedBR)
					i = end
					continue
				}
				if end, linkMD, ok := matchMarkdownAutolink(seg, i); ok {
					out.WriteString(linkMD)
					i = end
					continue
				}
				if opts.AllowXMLTags {
					out.WriteByte('<')
					i++
					continue
				}
				out.WriteString(`\<`)
				i++
				continue
			}

			out.WriteByte(seg[i])
			i++
		}
	}

	return out.String(), resources, nil
}

// findMathSpanEnd returns the byte offset immediately after a closing $ or $$
// span starting at start (where seg[start] == '$'). Escaped \$ inside math
// does not close the span.
func findMathSpanEnd(seg string, start int) (int, bool) {
	if start >= len(seg) || seg[start] != '$' {
		return 0, false
	}
	if start+1 < len(seg) && seg[start+1] == '$' {
		for j := start + 2; j < len(seg); {
			if seg[j] == '\\' {
				if j+1 < len(seg) {
					j += 2
				} else {
					j++
				}
				continue
			}
			if seg[j] == '$' && j+1 < len(seg) && seg[j+1] == '$' {
				if j == start+2 {
					return 0, false
				}
				return j + 2, true
			}
			j++
		}
		return 0, false
	}

	// Inline $...$: opening delimiter must not be followed by whitespace/newline.
	if start+1 >= len(seg) || seg[start+1] == ' ' || seg[start+1] == '\t' || seg[start+1] == '\n' {
		return 0, false
	}
	for j := start + 1; j < len(seg); {
		if seg[j] == '\n' && j+1 < len(seg) && seg[j+1] == '\n' {
			return 0, false
		}
		if seg[j] == '\\' {
			if j+1 < len(seg) {
				j += 2
			} else {
				j++
			}
			continue
		}
		if seg[j] == '$' {
			if j+1 < len(seg) && seg[j+1] == '$' {
				return 0, false
			}
			prev := seg[j-1]
			if prev == ' ' || prev == '\t' || prev == '\n' {
				return 0, false
			}
			if j+1 < len(seg) && seg[j+1] >= '0' && seg[j+1] <= '9' {
				return 0, false
			}
			return j + 1, true
		}
		j++
	}
	return 0, false
}

func matchHTMLBreakTag(seg string, start int) (int, string, bool) {
	rest := seg[start:]
	if len(rest) < 4 || (rest[0] != '<') {
		return 0, "", false
	}
	closeIdx := strings.IndexByte(rest, '>')
	if closeIdx < 0 || closeIdx > 12 {
		return 0, "", false
	}
	inner := strings.TrimSpace(rest[1:closeIdx])
	if strings.EqualFold(inner, "br") || strings.EqualFold(inner, "br/") || strings.EqualFold(inner, "br /") {
		return start + closeIdx + 1, "<br/>", true
	}
	return 0, "", false
}

func matchMarkdownAutolink(seg string, start int) (int, string, bool) {
	rest := seg[start+1:]
	nl := strings.IndexByte(rest, '\n')
	closeIdx := strings.IndexByte(rest, '>')
	if closeIdx <= 0 || (nl >= 0 && nl < closeIdx) {
		return 0, "", false
	}
	inner := rest[:closeIdx]
	if strings.ContainsAny(inner, " \t\r<") {
		return 0, "", false
	}
	lower := strings.ToLower(inner)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return start + 1 + closeIdx + 1, fmt.Sprintf("[%s](%s)", inner, inner), true
	case strings.HasPrefix(lower, "mailto:"):
		addr := inner[len("mailto:"):]
		if addr == "" {
			return 0, "", false
		}
		return start + 1 + closeIdx + 1, fmt.Sprintf("[%s](%s)", addr, inner), true
	default:
		return 0, "", false
	}
}

func compileMarkdownImage(rawSpan string, img parsedMarkdownImage, opts PushMarkdownOptions, occurrence int) (string, localDocResource, bool, error) {
	dest := strings.TrimSpace(img.Destination)
	if len(dest) >= 2 && dest[0] == '<' && dest[len(dest)-1] == '>' {
		dest = strings.TrimSpace(dest[1 : len(dest)-1])
	}
	lower := strings.ToLower(dest)
	if dest == "" || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return rawSpan, localDocResource{}, false, nil
	}

	dest = strings.TrimPrefix(dest, "@")
	resolvedPath, err := resolvePushLocalImagePath(dest, opts.BaseDir, occurrence)
	if err != nil {
		return "", localDocResource{}, false, err
	}
	info, err := vfs.Stat(resolvedPath)
	if err != nil {
		return "", localDocResource{}, false, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"local image #%d (%q) cannot be accessed: %v",
			occurrence, resolvedPath, err,
		).WithParam("file").WithCause(err)
	}
	if !info.Mode().IsRegular() {
		return "", localDocResource{}, false, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"local image #%d (%q) must point to a regular file",
			occurrence, resolvedPath,
		).WithParam("file")
	}
	if info.Size() <= 0 {
		return "", localDocResource{}, false, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"local image #%d (%q) must not be empty",
			occurrence, resolvedPath,
		).WithParam("file")
	}
	if info.Size() > common.MaxDriveMediaUploadSinglePartSize {
		return "", localDocResource{}, false, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"local image #%d (%q) exceeds 20 MiB upload limit",
			occurrence, resolvedPath,
		).WithParam("file")
	}

	contentBytes, err := vfs.ReadFile(resolvedPath)
	if err != nil {
		return "", localDocResource{}, false, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"local image #%d (%q) is not readable: %v",
			occurrence, resolvedPath, err,
		).WithParam("file").WithCause(err)
	}
	cfg, _, err := imageconfig.Decode(bytes.NewReader(contentBytes))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		if err == nil {
			err = invalidLocalDocImageDimensionsError()
		}
		return "", localDocResource{}, false, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"local image #%d (%q) is not a supported BMP, GIF, JPEG, PNG, TIFF, or WebP image: %v",
			occurrence, resolvedPath, err,
		).WithParam("file").WithCause(err)
	}

	marker, err := newLocalDocResourceMarker(localDocResourceImage)
	if err != nil {
		return "", localDocResource{}, false, errs.NewInternalError(errs.SubtypeUnknown, "failed to generate image marker").WithCause(err)
	}

	var tag bytes.Buffer
	tag.WriteString(`<img path="`)
	tag.WriteString(escapeXMLAttr(marker))
	tag.WriteByte('"')
	if alt := strings.TrimSpace(img.Alt); alt != "" {
		tag.WriteString(` caption="`)
		tag.WriteString(escapeXMLAttr(unescapeMarkdownText(alt)))
		tag.WriteByte('"')
	}
	if title := strings.TrimSpace(img.Title); title != "" {
		tag.WriteString(` title="`)
		tag.WriteString(escapeXMLAttr(title))
		tag.WriteByte('"')
	}
	tag.WriteString("/>")

	resource := localDocResource{
		Occurrence:  occurrence,
		Kind:        localDocResourceImage,
		Marker:      marker,
		Path:        resolvedPath,
		FileName:    filepath.Base(resolvedPath),
		Size:        int64(len(contentBytes)),
		ImageWidth:  cfg.Width,
		ImageHeight: cfg.Height,
		Content:     contentBytes,
	}
	return tag.String(), resource, true, nil
}

func resolvePushLocalImagePath(rawTarget, baseDir string, occurrence int) (string, error) {
	target := strings.TrimSpace(rawTarget)
	if strings.HasPrefix(strings.ToLower(target), "file://") {
		u, err := url.Parse(target)
		if err != nil {
			return "", errs.NewValidationError(
				errs.SubtypeInvalidArgument,
				"local image #%d has invalid file:// URL %q: %v",
				occurrence, rawTarget, err,
			).WithParam("file").WithCause(err)
		}
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			return "", errs.NewValidationError(
				errs.SubtypeInvalidArgument,
				"local image #%d file:// URL must not specify remote host %q",
				occurrence, u.Host,
			).WithParam("file")
		}
		target = filepath.FromSlash(u.Path)
	} else {
		if unescaped, err := url.PathUnescape(target); err == nil {
			target = unescaped
		}
	}

	if target == "~" || strings.HasPrefix(target, "~/") || strings.HasPrefix(target, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "cannot resolve home directory for %q: %v", rawTarget, err).WithParam("file").WithCause(err)
		}
		if target == "~" {
			target = home
		} else {
			target = filepath.Join(home, target[2:])
		}
	}

	var candidates []string
	if filepath.IsAbs(target) {
		candidates = append(candidates, target)
	} else {
		if baseDir != "" {
			candidates = append(candidates, filepath.Join(baseDir, target))
		}
		if cwd, err := vfs.Getwd(); err == nil && (baseDir == "" || filepath.Clean(cwd) != filepath.Clean(baseDir)) {
			candidates = append(candidates, filepath.Join(cwd, target))
		}
		if len(candidates) == 0 {
			candidates = append(candidates, target)
		}
	}

	var lastSecurityErr error
	for i, cand := range candidates {
		clean, err := localfileio.LocalInputPath(cand)
		if err != nil {
			lastSecurityErr = errs.NewValidationError(
				errs.SubtypeInvalidArgument,
				"local image #%d path %q is blocked by local file policy: %v",
				occurrence, rawTarget, err,
			).WithParam("file").WithCause(err)
			if i == 0 && len(candidates) > 1 {
				// If primary baseDir is safe or denied, keep checking? No: never bypass policy if primary hits deny root; fail fast!
				return "", lastSecurityErr
			}
			continue
		}
		if _, err := vfs.Stat(clean); err == nil {
			return clean, nil
		}
	}
	if lastSecurityErr != nil {
		return "", lastSecurityErr
	}
	clean, err := localfileio.LocalInputPath(candidates[0])
	if err != nil {
		return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "local image #%d path %q is invalid: %v", occurrence, rawTarget, err).WithParam("file").WithCause(err)
	}
	return clean, nil
}

func stripLeadingDocTitleTag(s string) (string, string) {
	trimmed := strings.TrimLeft(s, " \t\n")
	if !strings.HasPrefix(strings.ToLower(trimmed), "<title>") {
		return "", s
	}
	end := strings.Index(strings.ToLower(trimmed), "</title>")
	if end < 0 {
		return "", s
	}
	inner := html.UnescapeString(trimmed[len("<title>"):end])
	inner = strings.TrimSpace(strings.ReplaceAll(inner, "\n", " "))
	rest := trimmed[end+len("</title>"):]
	rest = strings.TrimLeft(rest, "\n")
	return inner, rest
}

func unescapePulledMarkdownProse(seg string) string {
	var out strings.Builder
	for i := 0; i < len(seg); {
		if seg[i] == '`' {
			run := countByteRun(seg, i, '`')
			if end := findMatchingBacktickRun(seg, i+run, run); end > 0 {
				out.WriteString(seg[i:end])
				i = end
				continue
			}
			out.WriteString(seg[i : i+run])
			i += run
			continue
		}
		if seg[i] == '$' {
			if end, ok := findMathSpanEnd(seg, i); ok {
				out.WriteString(seg[i:end])
				i = end
				continue
			}
			out.WriteByte('$')
			i++
			continue
		}
		if seg[i] == '\\' && i+1 < len(seg) {
			next := seg[i+1]
			switch next {
			case '\\', '$':
				// Keep escaped backslashes and literal currency \$ intact.
				out.WriteByte('\\')
				out.WriteByte(next)
				i += 2
			case '.', '-', '*', '_', '[', ']', '(', ')', '#', '+', '!', '|', '<', '>', '~':
				out.WriteByte(next)
				i += 2
			default:
				out.WriteByte('\\')
				out.WriteByte(next)
				i += 2
			}
			continue
		}
		out.WriteByte(seg[i])
		i++
	}
	return out.String()
}

func collapseExcessBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

func startsWithMatchingH1(body, title string) bool {
	firstLine := body
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		firstLine = body[:nl]
	}
	h1, _ := extractLeadingH1(firstLine + "\n")
	return h1 != "" && strings.EqualFold(strings.TrimSpace(h1), strings.TrimSpace(title))
}
