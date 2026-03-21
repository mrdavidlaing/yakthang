package zellij

import (
	"strings"
	"testing"

	"github.com/wellmaintained/yakthang/src/yak-box/pkg/types"
)

func TestGenerateLayout_Sandboxed(t *testing.T) {
	worker := &types.Worker{DisplayName: "My Worker", CWD: "/workspace"}
	out := GenerateLayout(worker, "sandboxed", "claude")
	if !strings.Contains(out, "My Worker") {
		t.Error("DisplayName should appear")
	}
	// Sandboxed layout has no tab cwd; CWD may appear in shell pane name for container
	// Pane name is tool (build) [runtimeKind]
	if !strings.Contains(out, "claude") || !strings.Contains(out, "build") || !strings.Contains(out, "sandbox") {
		t.Error("sandboxed layout should have tool and runtime in pane name")
	}
	if !strings.Contains(out, "WRAPPER") {
		t.Error("layout should contain WRAPPER placeholder")
	}
	if !strings.Contains(out, "SHELL_EXEC_SCRIPT") {
		t.Error("sandboxed layout should contain SHELL_EXEC_SCRIPT placeholder")
	}
	if !strings.Contains(out, "compact-bar") {
		t.Error("layout should include compact-bar plugin")
	}
	// Shell pane should have size=5
	if !strings.Contains(out, `pane size=5 name="shell:`) {
		t.Error("shell pane should have size=5")
	}
	// Main build pane should flex-fill (no size= attribute)
	if !strings.Contains(out, `pane name="claude (build) [sandboxed]" focus=true`) {
		t.Error("main build pane should not have a size= attribute")
	}
}

func TestGenerateLayout_Native(t *testing.T) {
	worker := &types.Worker{DisplayName: "Native Worker", CWD: "/home/proj"}
	out := GenerateLayout(worker, "native", "opencode")
	if !strings.Contains(out, "Native Worker") {
		t.Error("DisplayName should appear")
	}
	if !strings.Contains(out, "/home/proj") {
		t.Error("CWD should appear")
	}
	// Native layout includes cwd= in tab and pane name
	if !strings.Contains(out, `cwd="/home/proj"`) {
		t.Error("native layout should set tab cwd")
	}
	if !strings.Contains(out, "opencode (build) [native]") {
		t.Error("layout should have build pane name with runtime")
	}
	// Native layout should NOT have a secondary shell pane
	if strings.Contains(out, `pane size=5 name="shell:`) {
		t.Error("native layout should not have a secondary shell pane")
	}
	// Main build pane should flex-fill (no size= attribute)
	if !strings.Contains(out, `pane name="opencode (build) [native]" focus=true`) {
		t.Error("main build pane should not have a size= attribute")
	}
}
