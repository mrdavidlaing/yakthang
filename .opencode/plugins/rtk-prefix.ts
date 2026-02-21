import type { Plugin } from "@opencode-ai/plugin"

const RTK_COMMANDS = new Set([
  "git",
  "cargo",
  "npm",
  "yarn",
  "pnpm",
  "npx",
  "pytest",
  "python",
  "go",
  "ls",
  "find",
  "grep",
  "rg",
  "ag",
  "make",
  "docker",
  "kubectl",
])

export const RtkPrefixPlugin: Plugin = async ({ $ }) => {
  let rtkAvailable = false
  try {
    const result = await $`which rtk`.quiet().nothrow()
    rtkAvailable = result.exitCode === 0
  } catch {
    rtkAvailable = false
  }

  if (!rtkAvailable) {
    console.warn("[rtk-prefix] rtk not found in PATH — plugin inactive")
    return {}
  }

  return {
    "tool.execute.before": async (input, output) => {
      if (input.tool !== "bash") return

      const command: string = output.args?.command ?? ""
      if (!command) return

      const firstToken = command
        .replace(/^\s*(?:[A-Z_][A-Z0-9_]*=\S+\s+)*/, "")
        .trimStart()
        .split(/\s+/)[0]
        ?.replace(/^.*\//, "")

      if (!firstToken || !RTK_COMMANDS.has(firstToken)) return
      if (command.trimStart().startsWith("rtk ")) return

      output.args.command = `rtk ${command}`
    },
  }
}
