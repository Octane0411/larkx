// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
	"github.com/larksuite/cli/internal/vfs/localfileio"
	"github.com/larksuite/cli/shortcuts/common"
)

// DocsPull implements `larkx docs +pull` (alias `larkx docs pull`).
var DocsPull = common.Shortcut{
	Service:           "docs",
	Command:           "+pull",
	Description:       "Pull a Lark document as clean standard Markdown",
	Risk:              "read",
	AuthTypes:         []string{"user", "bot"},
	Scopes:            []string{"docx:document:readonly"},
	ConditionalScopes: []string{"docs:document.media:download", common.DrivePermissionMemberAuthScope},
	PostMount:         configureDocsPullCommand,
	Flags: []common.Flag{
		{Name: "doc", Desc: "document URL or token (also accepted as positional arg: larkx docs pull <url|token>)"},
		{Name: "output", Shorthand: "o", Desc: "write Markdown to local file path; omit to print Markdown to stdout"},
		{Name: "download-media", Desc: "download embedded document images locally and rewrite Markdown links", Type: "bool", Default: "false"},
		{Name: "media-dir", Desc: "directory for downloaded images (defaults to <output-stem>.assets or .larkx-media/<token>)"},
		{Name: "raw", Desc: "keep raw docs_ai markdown without unescaping or '# Title' normalization", Type: "bool", Default: "false"},
		{Name: "revision-id", Desc: "document revision id; -1 means latest", Type: "int", Default: "-1"},
		{Name: "lang", Desc: "user cite display language, e.g. en-US, zh-CN, ja-JP"},
	},
	Validate: validateDocsPull,
	DryRun:   dryRunDocsPull,
	Execute:  executeDocsPull,
}

func configureDocsPullCommand(cmd *cobra.Command) {
	cmd.Aliases = []string{"pull"}
	cmd.Args = cobra.MaximumNArgs(1)
	prevPreRunE := cmd.PreRunE
	cmd.PreRunE = func(command *cobra.Command, args []string) error {
		if len(args) == 1 {
			if command.Flags().Changed("doc") {
				return errs.NewValidationError(
					errs.SubtypeInvalidArgument,
					"cannot pass positional <url|token> together with --doc",
				).WithParam("--doc")
			}
			if err := command.Flags().Set("doc", args[0]); err != nil {
				return err
			}
		}
		if prevPreRunE != nil {
			return prevPreRunE(command, args)
		}
		return nil
	}
}

func validateDocsPull(_ context.Context, runtime *common.RuntimeContext) error {
	docTarget := strings.TrimSpace(runtime.Str("doc"))
	if docTarget == "" {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"document URL or token is required (larkx docs pull <url|token> or --doc <url|token>)",
		).WithParam("--doc")
	}
	if _, err := parseDocumentRef(docTarget); err != nil {
		return err
	}
	if outPath := strings.TrimSpace(runtime.Str("output")); outPath != "" {
		if _, err := localfileio.LocalInputPath(outPath); err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --output path %q: %v", outPath, err).WithParam("--output").WithCause(err)
		}
	}
	if mediaDir := strings.TrimSpace(runtime.Str("media-dir")); mediaDir != "" {
		if _, err := localfileio.LocalInputPath(mediaDir); err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --media-dir path %q: %v", mediaDir, err).WithParam("--media-dir").WithCause(err)
		}
	}
	if runtime.Bool("download-media") {
		if err := runtime.EnsureScopes([]string{"docs:document.media:download"}); err != nil {
			return err
		}
	}
	return nil
}

func dryRunDocsPull(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	ref, _ := parseDocumentRef(runtime.Str("doc"))
	body := buildDocsPullFetchBody(runtime)
	apiPath := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s/fetch", validate.EncodePathSegment(ref.Token))
	dry := common.NewDryRunAPI().
		POST(apiPath).
		Desc("OpenAPI: fetch document as markdown").
		Body(body).
		Set("document_id", ref.Token)
	if runtime.Bool("download-media") {
		dry.GET("/open-apis/drive/v1/medias/<media_token>/download").
			Desc("Conditional: download embedded document media files locally")
	}
	return dry
}

func executeDocsPull(_ context.Context, runtime *common.RuntimeContext) error {
	ref, _ := parseDocumentRef(runtime.Str("doc"))
	apiPath := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s/fetch", validate.EncodePathSegment(ref.Token))
	body := buildDocsPullFetchBody(runtime)

	data, err := doDocAPI(runtime, "POST", apiPath, body)
	if err != nil {
		return err
	}
	if err := processHTML5BlockReferenceMapForFetch(runtime, "markdown", ref.Token, data); err != nil {
		return err
	}

	docMap, _ := data["document"].(map[string]interface{})
	docID := strings.TrimSpace(common.GetString(docMap, "document_id"))
	if docID == "" {
		docID = ref.Token
	}
	docTitle := strings.TrimSpace(common.GetString(docMap, "title"))
	rawContent := common.GetString(docMap, "content")

	outPath := strings.TrimSpace(runtime.Str("output"))
	var cleanOut string
	if outPath != "" {
		cleanOut, err = localfileio.LocalInputPath(outPath)
		if err != nil {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --output path %q: %v", outPath, err).WithParam("--output").WithCause(err)
		}
	}

	md := NormalizePulledMarkdown(docTitle, rawContent, runtime.Bool("raw"))

	mediaCount := 0
	var mediaFiles []string
	if runtime.Bool("download-media") {
		mediaDir, dirErr := resolvePullMediaDir(runtime, cleanOut, docID)
		if dirErr != nil {
			return dirErr
		}
		var rewritten string
		rewritten, mediaFiles, err = downloadAndRewritePullMedia(runtime, md, cleanOut, mediaDir)
		if err != nil {
			return err
		}
		md = rewritten
		mediaCount = len(mediaFiles)
	}

	if cleanOut != "" {
		if err := vfs.MkdirAll(filepath.Dir(cleanOut), 0o755); err != nil {
			return errs.NewInternalError(errs.SubtypeUnknown, "create output directory failed: %v", err).WithCause(err)
		}
		if err := localfileio.AtomicWrite(cleanOut, []byte(md), 0o644); err != nil {
			return errs.NewInternalError(errs.SubtypeUnknown, "write output file failed: %v", err).WithCause(err)
		}
		result := map[string]interface{}{
			"document_id":      docID,
			"title":            docTitle,
			"output":           cleanOut,
			"bytes":            len(md),
			"media_downloaded": mediaCount,
		}
		if len(mediaFiles) > 0 {
			result["media_files"] = mediaFiles
		}
		runtime.Out(result, nil)
		return nil
	}

	if runtime.Cmd != nil && runtime.Cmd.Flags().Changed("format") && runtime.Format == "json" {
		result := map[string]interface{}{
			"document_id":      docID,
			"title":            docTitle,
			"content":          md,
			"media_downloaded": mediaCount,
		}
		if len(mediaFiles) > 0 {
			result["media_files"] = mediaFiles
		}
		runtime.OutRaw(result, nil)
		return nil
	}

	if _, writeErr := io.WriteString(runtime.IO().Out, md); writeErr != nil {
		return errs.NewInternalError(errs.SubtypeUnknown, "write stdout failed: %v", writeErr).WithCause(writeErr)
	}
	return nil
}

func buildDocsPullFetchBody(runtime *common.RuntimeContext) map[string]interface{} {
	body := map[string]interface{}{
		"format":      "markdown",
		"extra_param": docsFetchExtraParam,
		"export_option": map[string]interface{}{
			"export_block_id":        false,
			"export_style_attrs":     false,
			"export_cite_extra_data": false,
		},
	}
	if v := runtime.Int("revision-id"); v > 0 {
		body["revision_id"] = v
	}
	if lang := strings.TrimSpace(runtime.Str("lang")); lang != "" {
		body["lang"] = lang
	}
	injectDocsScene(runtime, body)
	return body
}

func resolvePullMediaDir(runtime *common.RuntimeContext, cleanOut, docID string) (string, error) {
	if explicit := strings.TrimSpace(runtime.Str("media-dir")); explicit != "" {
		clean, err := localfileio.LocalInputPath(explicit)
		if err != nil {
			return "", errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --media-dir %q: %v", explicit, err).WithParam("--media-dir").WithCause(err)
		}
		return clean, nil
	}
	if cleanOut != "" {
		stem := strings.TrimSuffix(cleanOut, filepath.Ext(cleanOut))
		return stem + ".assets", nil
	}
	return filepath.Join(".larkx-media", sanitizeMediaToken(docID)), nil
}

func downloadAndRewritePullMedia(runtime *common.RuntimeContext, md, cleanOut, mediaDir string) (string, []string, error) {
	mdBaseDir := "."
	if cleanOut != "" {
		mdBaseDir = filepath.Dir(cleanOut)
	} else if cwd, err := vfs.Getwd(); err == nil {
		mdBaseDir = cwd
	}

	downloaded := make(map[string]string)
	var savedFiles []string

	chunks := splitMarkdownFenceChunks(md)
	var out strings.Builder
	for _, chunk := range chunks {
		if chunk.isFence {
			out.WriteString(chunk.text)
			continue
		}
		rewritten, err := rewritePullMediaProse(runtime, chunk.text, mdBaseDir, mediaDir, downloaded, &savedFiles)
		if err != nil {
			return "", nil, err
		}
		out.WriteString(rewritten)
	}
	return out.String(), savedFiles, nil
}

func rewritePullMediaProse(
	runtime *common.RuntimeContext,
	seg string,
	mdBaseDir string,
	mediaDir string,
	downloaded map[string]string,
	savedFiles *[]string,
) (string, error) {
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
		if seg[i] == '<' && isPullImgTagStart(seg, i) {
			if tagEnd, ok := findHTMLTagEndOutsideQuotes(seg, i); ok {
				rawTag := seg[i:tagEnd]
				afterTag := tagEnd
				if !strings.HasSuffix(strings.TrimSpace(rawTag), "/>") && strings.HasPrefix(strings.ToLower(seg[afterTag:]), "</img>") {
					afterTag += len("</img>")
				}
				replacement, matched, err := tryReplacePullImgTag(runtime, rawTag, mdBaseDir, mediaDir, downloaded, savedFiles)
				if err != nil {
					return "", err
				}
				if matched {
					out.WriteString(replacement)
					i = afterTag
					continue
				}
			}
		}
		out.WriteByte(seg[i])
		i++
	}
	return out.String(), nil
}

func isPullImgTagStart(s string, idx int) bool {
	if idx+4 > len(s) {
		return false
	}
	if !strings.EqualFold(s[idx:idx+4], "<img") {
		return false
	}
	if idx+4 == len(s) {
		return false
	}
	next := s[idx+4]
	return next == ' ' || next == '\t' || next == '\n' || next == '>' || next == '/'
}

func findHTMLTagEndOutsideQuotes(s string, start int) (int, bool) {
	var quote byte
	for i := start + 1; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '>' {
			return i + 1, true
		}
	}
	return 0, false
}

func tryReplacePullImgTag(
	runtime *common.RuntimeContext,
	rawTag string,
	mdBaseDir string,
	mediaDir string,
	downloaded map[string]string,
	savedFiles *[]string,
) (string, bool, error) {
	xmlCandidate := strings.TrimSpace(rawTag)
	if !strings.HasSuffix(xmlCandidate, "/>") && strings.HasSuffix(xmlCandidate, ">") {
		xmlCandidate = strings.TrimSuffix(xmlCandidate, ">") + "/>"
	}
	tag, err := parseLocalDocResourceTag(xmlCandidate, "img")
	if err != nil {
		return "", false, nil
	}
	token := firstNonEmptyTagAttr(tag, "token", "src", "file_token", "img_key", "href")
	lowerTok := strings.ToLower(token)
	if token == "" || strings.HasPrefix(lowerTok, "http://") || strings.HasPrefix(lowerTok, "https://") || strings.HasPrefix(lowerTok, "data:") || strings.HasPrefix(token, "@") {
		return "", false, nil
	}
	alt := firstNonEmptyTagAttr(tag, "alt", "caption", "title", "name")

	localPath, ok := downloaded[token]
	if !ok {
		apiPath := fmt.Sprintf("/open-apis/drive/v1/medias/%s/download", validate.EncodePathSegment(token))
		resp, err := runtime.DoAPIStream(runtime.Ctx(), &larkcore.ApiReq{
			HttpMethod: http.MethodGet,
			ApiPath:    apiPath,
		})
		if err != nil {
			return "", false, withDocMediaDownloadRecoveryHint(wrapDocNetworkErr(err, "download media %q failed: %v", token, err), "media")
		}
		imgBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, common.MaxDriveMediaUploadSinglePartSize+1))
		closeErr := resp.Body.Close()
		if readErr != nil {
			return "", false, errs.NewInternalError(errs.SubtypeUnknown, "read downloaded media %q failed: %v", token, readErr).WithCause(readErr)
		}
		if closeErr != nil {
			return "", false, errs.NewInternalError(errs.SubtypeUnknown, "close downloaded media stream %q failed: %v", token, closeErr).WithCause(closeErr)
		}
		if int64(len(imgBytes)) > common.MaxDriveMediaUploadSinglePartSize {
			return "", false, errs.NewValidationError(errs.SubtypeInvalidArgument, "downloaded media %q exceeds 20 MiB", token)
		}

		basePath := filepath.Join(mediaDir, sanitizeMediaToken(token))
		finalPath, resolution := autoAppendDocMediaExtension(basePath, resp.Header, "")
		if resolution == nil && filepath.Ext(finalPath) == "" {
			finalPath += sniffImageExtension(imgBytes)
		}
		if err := vfs.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
			return "", false, errs.NewInternalError(errs.SubtypeUnknown, "create media directory failed: %v", err).WithCause(err)
		}
		if err := localfileio.AtomicWrite(finalPath, imgBytes, 0o644); err != nil {
			return "", false, errs.NewInternalError(errs.SubtypeUnknown, "write media file %q failed: %v", finalPath, err).WithCause(err)
		}
		localPath = finalPath
		downloaded[token] = localPath
		*savedFiles = append(*savedFiles, localPath)
	}

	linkTarget := localPath
	if rel, relErr := filepath.Rel(mdBaseDir, localPath); relErr == nil {
		linkTarget = rel
	}
	linkTarget = filepath.ToSlash(linkTarget)
	escapedAlt := strings.ReplaceAll(alt, "]", `\]`)
	return fmt.Sprintf("![%s](%s)", escapedAlt, linkTarget), true, nil
}

func firstNonEmptyTagAttr(tag localDocResourceTag, names ...string) string {
	for _, name := range names {
		if val, ok := tag.attr(name); ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}

func sanitizeMediaToken(token string) string {
	var b strings.Builder
	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	res := strings.Trim(b.String(), "_")
	if res == "" {
		return "media"
	}
	return res
}

func sniffImageExtension(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".bin"
	}
}
