// Package zellij provides Zellij terminal layout generation for yak-box.
package zellij

import (
	"fmt"

	"github.com/yakthang/yakbox/pkg/types"
)

// GenerateLayout generates a KDL layout file for a worker
func GenerateLayout(worker *types.Worker, runtime string) string {
	if runtime == "sandboxed" {
		return fmt.Sprintf(`layout {
    tab name="%s" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane size="67%%" name="opencode (build) [docker]" focus=true {
            command "bash"
            args "%%WRAPPER%%"
        }
        pane size="33%%" name="shell: %s"
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, worker.CWD)
	}

	return fmt.Sprintf(`layout {
    tab name="%s" cwd="%s" {
        pane size=1 borderless=true {
            plugin location="compact-bar"
        }
        pane size="67%%" name="opencode (build)" focus=true {
            command "bash"
            args "%%WRAPPER%%"
        }
        pane size="33%%" name="shell: %s"
        pane size=2 borderless=true {
            plugin location="status-bar"
        }
    }
}
`, worker.DisplayName, worker.CWD, worker.CWD)
}
