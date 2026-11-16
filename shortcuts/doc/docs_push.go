// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/internal/vfs"
	"github.com/larksuite/cli/internal/vfs/localfileio"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	docsPushStdinAnnotation = "larkx.docs.push.stdin"
)

var docsPushConditionalScopes = []string{
	"docx:document:create",
	"docx:document:write_only",
	"docx:document:readonly",
	"docs:document.media:upload",
	"wiki:node:retrieve",
}

// DocsPush implements `larkx docs +push` (alias `larkx docs push`).
var DocsPush = common.Shortcut{
	Service:           "docs",
	Command:           "+push",
	Description:       "Push standard Markdown to a Lark document",
	Risk:              "write",
	AuthTypes:         []string{"user", "bot"},
	ConditionalScopes: docsPushConditionalScopes,
	PostMount:         configureDocsPushCommand,
	Flags: []common.Flag{
		{Name: "file", Desc: "local Markdown file path (also accepted as positional arg); use '-' for stdin"},
		{Name: "content", Desc: "inline Markdown content (alternative to --file)", Input: []string{common.File, common.Stdin}},
		{Name: "doc", Desc: "existing document URL or token to update; omit to create a new document"},
		{Name: "mode", Desc: "update mode when --doc is set: overwrite | append", Default: "overwrite", Enum: []string{"overwrite", "append"}},
		{Name: "title", Desc: "explicit document title (defaults to YAML frontmatter title, first '# H1', or filename)"},
		{Name: "keep-h1", Desc: "keep leading '# H1' in document body instead of folding it solely into the document title", Type: "bool", Default: "false"},
		{Name: "keep-frontmatter", Desc: "keep YAML frontmatter in document body instead of stripping it", Type: "bool", Default: "false"},
		{Name: "allow-xml-tags", Desc: "pass raw DocxXML tags through instead of escaping bare '<'", Type: "bool", Default: "false"},
		{Name: "parent-token", Desc: "destination folder or wiki URL/token when creating a new document"},
		{Name: "parent-position", Desc: "position within parent node when creating a new document"},
	},
	Validate: validateDocsPush,
	DryRun:   dryRunDocsPush,
	Execute:  executeDocsPush,
}

func configureDocsPushCommand(cmd *cobra.Command) {
	cmd.Aliases = []string{"push"}
	cmd.Args = cobra.MaximumNArgs(1)
	prevPreRunE := cmd.PreRunE
	cmd.PreRunE = func(command *cobra.Command, args []string) error {
		if len(args) == 1 {
			if command.Flags().Changed("file") || command.Flags().Changed("content") {
				return errs.NewValidationError(
					errs.SubtypeInvalidArgument,
					"cannot pass positional <file.md> together with --file or --content",
				).WithParam("--file")
			}
			if err := command.Flags().Set("file", args[0]); err != nil {
				return err
			}
		}
		if prevPreRunE != nil {
			return prevPreRunE(command, args)
		}
		return nil
	}
}

func validateDocsPush(_ context.Context, runtime *common.RuntimeContext) error {
	fileFlag := strings.TrimSpace(runtime.Str("file"))
	contentChanged := runtime.Changed("content")
	fileChanged := runtime.Changed("file") || fileFlag != ""

	if fileChanged && contentChanged {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"--file and --content are mutually exclusive",
		).WithParam("--file")
	}
	if !fileChanged && !contentChanged {
		return errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"provide a Markdown file path (larkx docs push <file.md> or --file <path>) or --content",
		).WithParam("--file")
	}

	docTarget := strings.TrimSpace(runtime.Str("doc"))
	mode := runtime.Str("mode")
	if docTarget == "" {
		if runtime.Changed("mode") && mode == "append" {
			return errs.NewValidationError(
				errs.SubtypeInvalidArgument,
				"--mode append requires --doc",
			).WithParam("--mode")
		}
	} else {
		if _, err := parseDocumentRef(docTarget); err != nil {
			return err
		}
		if strings.TrimSpace(runtime.Str("parent-token")) != "" {
			return errs.NewValidationError(
				errs.SubtypeInvalidArgument,
				"--parent-token is only valid when creating a new document (without --doc)",
			).WithParam("--parent-token")
		}
		if strings.TrimSpace(runtime.Str("parent-position")) != "" {
			return errs.NewValidationError(
				errs.SubtypeInvalidArgument,
				"--parent-position is only valid when creating a new document (without --doc)",
			).WithParam("--parent-position")
		}
	}

	prepared, _, err := prepareDocsPushMarkdown(runtime)
	if err != nil {
		return err
	}

	if docTarget == "" {
		if strings.TrimSpace(prepared.FullCreateContent) == "" {
			return errs.NewValidationError(errs.SubtypeInvalidArgument, "markdown input is empty").WithParam("--file")
		}
		if err := runtime.EnsureScopes([]string{"docx:document:create"}); err != nil {
			return err
		}
		if len(prepared.Resources) > 0 {
			if err := runtime.EnsureScopes(docsCreateLocalResourceScopes); err != nil {
				return err
			}
		}
		return nil
	}

	if strings.TrimSpace(prepared.Body) == "" && prepared.Title == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "markdown input is empty").WithParam("--file")
	}
	ref, _ := parseDocumentRef(docTarget)
	if err := runtime.EnsureScopes([]string{"docx:document:write_only", "docx:document:readonly"}); err != nil {
		return err
	}
	if len(prepared.Resources) > 0 {
		if err := runtime.EnsureScopes(docsUpdateLocalResourceScopesFor(ref)); err != nil {
			return err
		}
	}
	return nil
}

func dryRunDocsPush(_ context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
	prepared, _, err := prepareDocsPushMarkdown(runtime)
	if err != nil {
		return common.NewDryRunAPI().Set("error", err.Error())
	}

	docTarget := strings.TrimSpace(runtime.Str("doc"))
	if docTarget == "" {
		body := buildDocsPushCreateBody(runtime, prepared)
		desc := "OpenAPI: create document from Markdown"
		if runtime.IsBot() {
			desc += ". After document creation succeeds in bot mode, the CLI will also try to grant the current CLI user full_access on the new document."
		}
		dry := common.NewDryRunAPI().
			POST("/open-apis/docs_ai/v1/documents").
			Desc(desc).
			Body(body)
		dry.GET("/open-apis/docs_ai/v1/async_tasks/<task_id>").
			Desc("Conditional: poll async task when document creation returns a task_id.")
		return appendLocalDocResourcesDryRun(dry, "<created_document_id>", prepared.Resources)
	}

	ref, _ := parseDocumentRef(docTarget)
	body := buildDocsPushUpdateBody(runtime, prepared)
	apiPath := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s", validate.EncodePathSegment(ref.Token))
	dry := common.NewDryRunAPI().
		PUT(apiPath).
		Desc(fmt.Sprintf("OpenAPI: update document from Markdown (--mode %s)", runtime.Str("mode"))).
		Body(body)
	return appendLocalDocResourcesDryRun(dry, "<resolved_document_id>", prepared.Resources)
}

func executeDocsPush(_ context.Context, runtime *common.RuntimeContext) error {
	prepared, _, err := prepareDocsPushMarkdown(runtime)
	if err != nil {
		return err
	}

	docTarget := strings.TrimSpace(runtime.Str("doc"))
	if docTarget == "" {
		return executeDocsPushCreate(runtime, prepared)
	}
	return executeDocsPushUpdate(runtime, docTarget, prepared)
}

func executeDocsPushCreate(runtime *common.RuntimeContext, prepared *PreparedMarkdownPush) error {
	trace := newDocsCreateTrace(runtime)
	var retErr error
	defer func() { trace.finish(retErr) }()

	trace.step("create_request")
	body := buildDocsPushCreateBody(runtime, prepared)
	data, createLogID, err := createDocsDocumentWithLogID(runtime, body)
	trace.event("create_response", docsCreateDebugDetails{LogID: createLogID, Resources: len(prepared.Resources)})
	if err != nil {
		retErr = err
		return err
	}
	if docsAPIOperationFailed(data) {
		retErr = runtime.OutPartialFailure(data, nil)
		return retErr
	}

	trace.step("wait_task")
	data, err = waitForDocsCreateAsyncTask(runtime, data, createLogID, trace)
	if err != nil {
		retErr = err
		return err
	}

	trace.step("permission")
	augmentDocsCreatePermission(runtime, data)
	trace.step("document_url")
	fallbackDocsCreateURLV2(runtime, data)

	trace.step("resources")
	if len(prepared.Resources) > 0 {
		doc, _ := data["document"].(map[string]interface{})
		if err := finalizeLocalDocResourcesWithTrace(runtime, strings.TrimSpace(common.GetString(doc, "document_id")), data, prepared.Resources, trace); err != nil {
			retErr = err
			return err
		}
	}

	if prepared.Title != "" {
		data["title"] = prepared.Title
	}
	data["images_uploaded"] = len(prepared.Resources)

	trace.step("output")
	runtime.OutRaw(data, nil)
	return nil
}

func executeDocsPushUpdate(runtime *common.RuntimeContext, docTarget string, prepared *PreparedMarkdownPush) error {
	ref, _ := parseDocumentRef(docTarget)
	documentID := ref.Token
	if len(prepared.Resources) > 0 {
		resolved, err := resolveDocxDocumentID(runtime, docTarget)
		if err != nil {
			return err
		}
		documentID = resolved
	}

	apiPath := fmt.Sprintf("/open-apis/docs_ai/v1/documents/%s", validate.EncodePathSegment(documentID))
	body := buildDocsPushUpdateBody(runtime, prepared)
	data, err := doDocAPI(runtime, "PUT", apiPath, body)
	if err != nil {
		return err
	}
	if docsAPIOperationFailed(data) {
		return runtime.OutPartialFailure(data, nil)
	}

	if err := finalizeLocalDocResources(runtime, documentID, data, prepared.Resources); err != nil {
		return err
	}
	data["document_id"] = documentID
	data["mode"] = runtime.Str("mode")
	data["images_uploaded"] = len(prepared.Resources)
	runtime.OutRaw(data, nil)
	return nil
}

func buildDocsPushCreateBody(runtime *common.RuntimeContext, prepared *PreparedMarkdownPush) map[string]interface{} {
	body := map[string]interface{}{
		"format":      "markdown",
		"content":     prepared.FullCreateContent,
		"extra_param": docsCreateAsyncExtraParam,
	}
	if v := normalizePushParentToken(runtime.Str("parent-token")); v != "" {
		body["parent_token"] = v
	}
	if v := strings.TrimSpace(runtime.Str("parent-position")); v != "" {
		body["parent_position"] = v
	}
	injectDocsScene(runtime, body)
	return body
}

func buildDocsPushUpdateBody(runtime *common.RuntimeContext, prepared *PreparedMarkdownPush) map[string]interface{} {
	mode := runtime.Str("mode")
	cmdName := "overwrite"
	content := prepared.Body
	body := map[string]interface{}{
		"format": "markdown",
	}
	if mode == "append" {
		cmdName = "block_insert_after"
		body["block_id"] = "-1"
	} else if strings.TrimSpace(content) == "" && prepared.Title != "" {
		content = "# " + prepared.Title
	}
	body["command"] = cmdName
	body["content"] = content
	injectDocsScene(runtime, body)
	return body
}

func prepareDocsPushMarkdown(runtime *common.RuntimeContext) (*PreparedMarkdownPush, string, error) {
	raw, sourcePath, baseDir, err := loadPushMarkdownInput(runtime)
	if err != nil {
		return nil, "", err
	}
	isUpdate := strings.TrimSpace(runtime.Str("doc")) != ""
	mode := runtime.Str("mode")
	stripH1 := (!isUpdate || mode == "overwrite") && !runtime.Bool("keep-h1")

	opts := PushMarkdownOptions{
		SourcePath:      sourcePath,
		BaseDir:         baseDir,
		TitleOverride:   runtime.Str("title"),
		KeepH1:          runtime.Bool("keep-h1"),
		KeepFrontmatter: runtime.Bool("keep-frontmatter"),
		AllowXMLTags:    runtime.Bool("allow-xml-tags"),
		StripLeadingH1:  stripH1,
		FallbackToStem:  !isUpdate,
	}
	prepared, err := PrepareMarkdownForPush(raw, opts)
	if err != nil {
		return nil, "", err
	}
	return prepared, sourcePath, nil
}

func loadPushMarkdownInput(runtime *common.RuntimeContext) (raw string, sourcePath string, baseDir string, err error) {
	fileFlag := strings.TrimSpace(runtime.Str("file"))
	if fileFlag == "-" {
		if runtime.Cmd != nil && runtime.Cmd.Annotations != nil {
			if cached, ok := runtime.Cmd.Annotations[docsPushStdinAnnotation]; ok {
				cwd, _ := vfs.Getwd()
				return cached, "-", cwd, nil
			}
		}
		data, readErr := io.ReadAll(runtime.IO().In)
		if readErr != nil {
			return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "failed reading stdin: %v", readErr).WithParam("--file").WithCause(readErr)
		}
		text := string(data)
		if runtime.Cmd != nil {
			if runtime.Cmd.Annotations == nil {
				runtime.Cmd.Annotations = make(map[string]string)
			}
			runtime.Cmd.Annotations[docsPushStdinAnnotation] = text
		}
		cwd, _ := vfs.Getwd()
		return text, "-", cwd, nil
	}

	if fileFlag != "" {
		cleanPath, pathErr := localfileio.LocalInputPath(fileFlag)
		if pathErr != nil {
			return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "invalid --file path %q: %v", fileFlag, pathErr).WithParam("--file").WithCause(pathErr)
		}
		info, statErr := vfs.Stat(cleanPath)
		if statErr != nil {
			return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "cannot access %q: %v", cleanPath, statErr).WithParam("--file").WithCause(statErr)
		}
		if info.IsDir() || !info.Mode().IsRegular() {
			return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "--file %q must point to a regular file", cleanPath).WithParam("--file")
		}
		data, readErr := vfs.ReadFile(cleanPath)
		if readErr != nil {
			return "", "", "", errs.NewValidationError(errs.SubtypeInvalidArgument, "cannot read %q: %v", cleanPath, readErr).WithParam("--file").WithCause(readErr)
		}
		absPath, absErr := filepath.Abs(cleanPath)
		if absErr != nil {
			absPath = cleanPath
		}
		return string(data), absPath, filepath.Dir(absPath), nil
	}

	cwd, _ := vfs.Getwd()
	return runtime.Str("content"), "", cwd, nil
}

func normalizePushParentToken(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		return trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(segments); i++ {
		if (segments[i] == "folder" || segments[i] == "wiki") && i+1 < len(segments) && segments[i+1] != "" {
			return segments[i+1]
		}
	}
	return trimmed
}
