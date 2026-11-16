// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package main

import "os"

// applyLarkxDefaults silences upstream update/skills sync notices and enables
// the self-update guard so `larkx update` does not overwrite the local build
// with an upstream release binary unless explicitly forced.
func applyLarkxDefaults() {
	if os.Getenv("LARKX_ENABLE_UPSTREAM_NOTIFIERS") != "1" {
		setEnvDefault("LARKSUITE_CLI_NO_UPDATE_NOTIFIER", "1")
		setEnvDefault("LARKSUITE_CLI_NO_SKILLS_NOTIFIER", "1")
	}
	setEnvDefault("LARKX_BINARY_GUARD", "1")
}

func setEnvDefault(key, value string) {
	if _, exists := os.LookupEnv(key); !exists {
		_ = os.Setenv(key, value)
	}
}
