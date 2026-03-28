// Package zellij provides Zellij terminal layout generation for yak-box.
// Layouts use placeholders so runtime can inject script paths: %%WRAPPER%%,
// and for devcontainer only %%SHELL_EXEC_SCRIPT%% and %%CONTAINER_NAME%%.
package zellij

import (
	"fmt"

	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

// GenerateLayout generates a KDL layout file for a worker.
// runtimeKind is the runtime type; tool is the worker tool (e.g. "claude", "cursor", "opencode").
// The returned string contains %%WRAPPER%%; for devcontainer it also contains %%SHELL_EXEC_SCRIPT%% and %%CONTAINER_NAME%%.
// Callers must replace these placeholders with actual paths before writing the layout file.
func GenerateLayout(worker *types.Worker, runtimeKind string, tool string) string {
	paneName := fmt.Sprintf("%s (build) [%s]", tool, runtimeKind)
	if types.Runtime(runtimeKind) == types.RuntimeDevcontainer {
		// Devcontainer: tab has no cwd; main pane runs wrapper; shell pane runs shell-exec script with container name.
		return fmt.Sprintf(`layout {
    tab name="%s" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane name="%s" focus=true {
            command "bash"
            args "%%WRAPPER%%"
        }
        pane size=5 name="shell: container" {
            command "bash"
            args "%%SHELL_EXEC_SCRIPT%%" "%%CONTAINER_NAME%%"
        }
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, paneName)
	}
	// Native: tab has cwd; single main pane with wrapper; tab closes when wrapper exits.
	return fmt.Sprintf(`layout {
    tab name="%s" cwd="%s" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane name="%s" focus=true {
            command "bash"
            args "%%WRAPPER%%"
        }
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, worker.CWD, paneName)
}
