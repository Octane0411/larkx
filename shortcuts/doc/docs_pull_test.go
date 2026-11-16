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
)

func TestDocsPullStdoutPrintsCleanMarkdown(t *testing.T) {
	t.Parallel()

	f, stdout, _, reg := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	f.Credential = credential.NewCredentialProvider(nil, nil, &docMediaScopedTokenResolver{scopes: "docx:document:readonly"}, nil)
	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/docs_ai/v1/documents/doxcnPull123/fetch",
		Status: 200,
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doxcnPull123",
					"content":     "<title>Clean Title</title>\n1\\. Item one\n",
				},
			},
		},
	})

	if err := mountAndRunDocs(t, DocsPull, []string{
		"pull", "https://example.feishu.cn/docx/doxcnPull123",
		"--as", "bot",
	}, f, stdout); err != nil {
		t.Fatalf("docs pull stdout error = %v", err)
	}
	want := "# Clean Title\n\n1. Item one\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestDocsPullOutputFileWithDownloadMedia(t *testing.T) {
	t.Parallel()

	f, stdout, _, reg := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	f.Credential = credential.NewCredentialProvider(nil, nil, &docMediaScopedTokenResolver{
		scopes: "docx:document:readonly docs:document.media:download",
	}, nil)

	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/docs_ai/v1/documents/doxcnPullMedia/fetch",
		Status: 200,
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doxcnPullMedia",
					"title":       "Architecture Spec",
					"content":     "See diagram:\n\n<img src=\"boxcnImgTok1\" alt=\"arch\"/>\n",
				},
			},
		},
	})
	pngBytes := []byte(localDocResourcePNG(t, 2, 2))
	reg.Register(&httpmock.Stub{
		Method:  "GET",
		URL:     "/open-apis/drive/v1/medias/boxcnImgTok1/download",
		Status:  200,
		RawBody: pngBytes,
		Headers: http.Header{"Content-Type": []string{"image/png"}},
	})

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "spec.md")
	if err := mountAndRunDocs(t, DocsPull, []string{
		"+pull",
		"--doc", "https://example.feishu.cn/docx/doxcnPullMedia",
		"-o", outFile,
		"--download-media",
		"--as", "bot",
	}, f, stdout); err != nil {
		t.Fatalf("docs +pull -o --download-media error = %v", err)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			DocumentID      string `json:"document_id"`
			Output          string `json:"output"`
			MediaDownloaded int    `json:"media_downloaded"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode json summary: %v\n%s", err, stdout.String())
	}
	if !envelope.OK || envelope.Data.MediaDownloaded != 1 || envelope.Data.DocumentID != "doxcnPullMedia" {
		t.Fatalf("unexpected pull summary envelope: %+v", envelope)
	}

	expectedMedia := filepath.Join(outDir, "spec.assets", "boxcnImgTok1.png")
	if _, err := os.Stat(expectedMedia); err != nil {
		t.Fatalf("expected downloaded media at %s: %v", expectedMedia, err)
	}

	written, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(written)
	if !strings.HasPrefix(text, "# Architecture Spec\n\n") {
		t.Errorf("written markdown missing normalized title header:\n%s", text)
	}
	if !strings.Contains(text, "![arch](spec.assets/boxcnImgTok1.png)") {
		t.Errorf("written markdown missing rewritten image link:\n%s", text)
	}
}

func TestDocsPullDownloadMediaRewritesFeishuFileMarkdownLinks(t *testing.T) {
	t.Parallel()

	f, stdout, _, reg := cmdutil.TestFactory(t, docsCreateTestConfig(t, ""))
	f.Credential = credential.NewCredentialProvider(nil, nil, &docMediaScopedTokenResolver{
		scopes: "docx:document:readonly docs:document.media:download",
	}, nil)

	reg.Register(&httpmock.Stub{
		Method: "POST",
		URL:    "/open-apis/docs_ai/v1/documents/doxcnPullLinks/fetch",
		Status: 200,
		Body: map[string]interface{}{
			"code": 0,
			"msg":  "ok",
			"data": map[string]interface{}{
				"document": map[string]interface{}{
					"document_id": "doxcnPullLinks",
					"content": strings.Join([]string{
						"<title>Links Doc</title>",
						"",
						"![图1](https://feishu.cn/file/XzYsbfPRFoGZxlxYnTMcx617ncc)",
						"![图2](https://example.larkoffice.com/file/BoxTok2)",
						"![ext](https://example.com/keep.png)",
					}, "\n"),
				},
			},
		},
	})
	pngBytes := []byte(localDocResourcePNG(t, 2, 2))
	for _, tok := range []string{"XzYsbfPRFoGZxlxYnTMcx617ncc", "BoxTok2"} {
		reg.Register(&httpmock.Stub{
			Method:  "GET",
			URL:     "/open-apis/drive/v1/medias/" + tok + "/download",
			Status:  200,
			RawBody: pngBytes,
			Headers: http.Header{"Content-Type": []string{"image/png"}},
		})
	}

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "links.md")
	if err := mountAndRunDocs(t, DocsPull, []string{
		"pull", "doxcnPullLinks",
		"-o", outFile,
		"--download-media",
		"--as", "bot",
	}, f, stdout); err != nil {
		t.Fatalf("docs pull --download-media error = %v", err)
	}

	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			MediaDownloaded int `json:"media_downloaded"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Data.MediaDownloaded != 2 {
		t.Fatalf("unexpected media_downloaded: %+v", envelope)
	}

	for _, tok := range []string{"XzYsbfPRFoGZxlxYnTMcx617ncc.png", "BoxTok2.png"} {
		if _, err := os.Stat(filepath.Join(outDir, "links.assets", tok)); err != nil {
			t.Fatalf("missing downloaded media %s: %v", tok, err)
		}
	}

	gotBytes, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	for _, want := range []string{
		"![图1](links.assets/XzYsbfPRFoGZxlxYnTMcx617ncc.png)",
		"![图2](links.assets/BoxTok2.png)",
		"![ext](https://example.com/keep.png)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("pulled markdown missing %q:\n%s", want, got)
		}
	}
}
