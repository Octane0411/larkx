// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/credential"
	"github.com/larksuite/cli/internal/httpmock"
	"github.com/larksuite/cli/shortcuts/common"
)

func TestDocsPushDryRunCreateWithExternalFileAndImage(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "a.png"), []byte(localDocResourcePNG(t, 2, 2)), 0o644); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(tmpDir, "report.md")
	mdContent := "# Weekly Report\n\nBudget \\$500 <USER_REQUEST>\n\n![pic](assets/a.png)\n"
	if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	f, stdout, _, _ := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	err := mountAndRunDocs(t, DocsPush, []string{
		"push", mdPath,
		"--parent-token", "https://example.feishu.cn/drive/folder/fldcnFolder123?from=space",
		"--dry-run",
		"--as", "bot",
	}, f, stdout)
	if err != nil {
		t.Fatalf("docs push --dry-run error = %v", err)
	}

	var envelope struct {
		Data struct {
			API []common.DryRunAPICall `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode dry-run envelope: %v\nraw=%s", err, stdout.String())
	}
	if len(envelope.Data.API) < 3 {
		t.Fatalf("dry-run API calls = %d, want create + async_tasks + upload: %#v", len(envelope.Data.API), envelope.Data.API)
	}
	first := envelope.Data.API[0]
	if first.Method != "POST" || first.URL != "/open-apis/docs_ai/v1/documents" {
		t.Fatalf("first call = %s %s, want POST /open-apis/docs_ai/v1/documents", first.Method, first.URL)
	}
	body, _ := first.Body.(map[string]interface{})
	if body["parent_token"] != "fldcnFolder123" {
		t.Fatalf("parent_token = %#v, want fldcnFolder123", body["parent_token"])
	}
	contentStr, _ := body["content"].(string)
	if !strings.HasPrefix(contentStr, "<title>Weekly Report</title>\n") {
		t.Fatalf("create content missing <title>: %q", contentStr)
	}
	if !strings.Contains(contentStr, `\<USER_REQUEST>`) || !strings.Contains(contentStr, `<img path="@lcli_img_`) {
		t.Fatalf("create content missing escaped angle or image marker: %q", contentStr)
	}
	if envelope.Data.API[2].URL != "/open-apis/drive/v1/medias/upload_all" {
		t.Fatalf("third API URL = %q, want upload_all", envelope.Data.API[2].URL)
	}
}

func TestDocsPushDryRunUpdateOverwriteAndAppend(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	if err := mountAndRunDocs(t, DocsPush, []string{
		"+push",
		"--doc", "https://example.feishu.cn/docx/doxcnTarget123",
		"--content", "# New Heading\n\nBody text.",
		"--dry-run",
		"--as", "bot",
	}, f, stdout); err != nil {
		t.Fatalf("docs +push overwrite dry-run error = %v", err)
	}
	var envOverwrite struct {
		Data struct {
			API []common.DryRunAPICall `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envOverwrite); err != nil {
		t.Fatal(err)
	}
	call := envOverwrite.Data.API[0]
	if call.Method != "PUT" || call.URL != "/open-apis/docs_ai/v1/documents/doxcnTarget123" {
		t.Fatalf("overwrite call = %s %s", call.Method, call.URL)
	}
	owBody, _ := call.Body.(map[string]interface{})
	if owBody["command"] != "overwrite" {
		t.Fatalf("command = %#v, want overwrite", owBody["command"])
	}

	if err := mountAndRunDocs(t, DocsPush, []string{
		"push",
		"--doc", "https://example.feishu.cn/docx/doxcnTarget123",
		"--mode", "append",
		"--content", "# Appended Section\n\nMore details.",
		"--dry-run",
		"--as", "bot",
	}, f, stdout); err != nil {
		t.Fatalf("docs push append dry-run error = %v", err)
	}
	var envAppend struct {
		Data struct {
			API []common.DryRunAPICall `json:"api"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envAppend); err != nil {
		t.Fatal(err)
	}
	apBody, _ := envAppend.Data.API[0].Body.(map[string]interface{})
	if apBody["command"] != "block_insert_after" || apBody["block_id"] != "-1" {
		t.Fatalf("append payload = %#v", apBody)
	}
	if contentStr, _ := apBody["content"].(string); !strings.Contains(contentStr, "# Appended Section") {
		t.Fatalf("append mode should keep leading '# Appended Section': %q", contentStr)
	}
}

func TestDocsPushRejectsConflictingFileArgs(t *testing.T) {
	t.Parallel()

	f, stdout, _, _ := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	err := mountAndRunDocs(t, DocsPush, []string{
		"push", "a.md", "--file", "b.md", "--as", "bot",
	}, f, stdout)
	if err == nil {
		t.Fatal("expected error when both positional file and --file are passed")
	}
}

func TestDocsPushExecuteMockedCreate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "note.md")
	if err := os.WriteFile(mdPath, []byte("# Shipped\n\nHello.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, stdout, _, reg := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	f.Credential = credential.NewCredentialProvider(nil, nil, &docMediaScopedTokenResolver{scopes: "docx:document:create"}, nil)
	var capturedBody map[string]interface{}
	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/docs_ai/v1/documents",
		Status: 200,
		OnMatch: func(req *http.Request) {
			_ = json.NewDecoder(req.Body).Decode(&capturedBody)
		},
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doxcnCreated123",
					"revision_id": 1,
					"url":         "https://example.feishu.cn/docx/doxcnCreated123",
				},
			},
		},
	})

	if err := mountAndRunDocs(t, DocsPush, []string{"push", mdPath, "--as", "bot"}, f, stdout); err != nil {
		t.Fatalf("docs push execute error = %v", err)
	}
	if capturedBody["format"] != "markdown" || capturedBody["content"] != "<title>Shipped</title>\nHello.\n" {
		t.Fatalf("captured request body = %#v", capturedBody)
	}
	env := decodeDocsCreateEnvelope(t, stdout)
	if env["title"] != "Shipped" {
		t.Fatalf("response title = %#v, want Shipped", env["title"])
	}
	if docMap, _ := env["document"].(map[string]interface{}); docMap["document_id"] != "doxcnCreated123" {
		t.Fatalf("response document = %#v", env["document"])
	}
}
