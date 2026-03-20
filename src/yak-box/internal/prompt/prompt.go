// Package prompt handles prompt generation and rendering for yak-box.
package prompt

import (
	"fmt"
	"strings"
)

// BuildPrompt assembles the initial prompt for a worker.
// shaverName is the identity of the shaver (persona in the prompt, e.g. "Yakoff"); skillNames are the skill folder basenames to reference.
func BuildPrompt(mode string, yakPath string, userPrompt string, tasks []string, shaverName string, skillNames []string) string {
	var roleDescription string
	if mode == "plan" {
		roleDescription = "Your supervisor is Yakob. The yaks are tasks — your job is to scout them and plan the shave. Do NOT pick up the clippers."
	} else {
		roleDescription = "Your supervisor is Yakob. The yaks are tasks — your job is to shave them clean."
	}
	if shaverName != "" {
		roleDescription = "You are " + shaverName + ". " + roleDescription
	}

	var workflow string
	if mode == "plan" {
		workflow = `Workflow:
1. Run 'yx ls' to see available tasks
2. Pick a task, read its context with 'yx context --show <name>'
3. Set it to wip: 'yx state <name> wip'
4. Report: echo "starting plan" | yx field <name> shaver-message
5. Analyze the codebase, understand the problem, and write a detailed plan
6. Save the plan where it makes sense (e.g. a markdown file, or in yx context)
7. Report: echo "plan ready for review" | yx field <name> shaver-message
8. STOP and wait — do NOT implement. Your job is to plan, not build.

Focus on the tasks assigned to you. Do not modify tasks outside your scope.`
	} else {
		workflow = `Workflow:
1. Run 'yx ls' to see available tasks
2. Pick a task, read its context with 'yx context --show <name>'
3. Set it to wip: 'yx state <name> wip'
4. Report: echo "starting work" | yx field <name> shaver-message
5. Do the work (update shaver-message as you make progress)
6. When done: echo "done — <summary>" | yx field <name> shaver-message
7. If blocked: echo "stuck: <reason>" | yx field <name> shaver-message

You do NOT control task state transitions — Yakob handles wip-state and done.
Focus on the tasks assigned to you. Do not modify tasks outside your scope.`
	}

	var taskAssignment string
	if len(tasks) > 0 {
		if len(tasks) == 1 {
			taskAssignment = fmt.Sprintf(`---
ASSIGNED TASK

You are assigned to work on: %s

Read the task details: yx context --show %s
Start working: yx state %s wip

---
TASK TRACKER (yx)`, tasks[0], tasks[0], tasks[0])
		} else {
			taskList := ""
			for _, t := range tasks {
				taskList += "  - " + t + "\n"
			}
			taskAssignment = fmt.Sprintf(`---
ASSIGNED TASKS

You are assigned to work on:
%s
Read each task's details: yx context --show <task>
Start working: yx state <task> wip

---
TASK TRACKER (yx)`, taskList)
		}
	} else {
		taskAssignment = `---
TASK TRACKER (yx)`
	}

	var skillSection string
	if len(skillNames) > 0 {
		var sb strings.Builder
		sb.WriteString("\n---\nSKILLS\n\nYou have been given the following skills to guide your work:\n")
		for _, name := range skillNames {
			fmt.Fprintf(&sb, "  - %s\n", name)
		}
		sb.WriteString("\nBEFORE DOING ANYTHING ELSE — before reading context, before writing code, before any action:\n")
		sb.WriteString("1. Invoke each skill listed above using the Skill tool (e.g. Skill(\"yx-task-management\")).\n")
		sb.WriteString("2. Respond with: \"I have read the skills: [list them]. I understand my task. Beginning now.\"\n")
		sb.WriteString("3. Only then begin work.\n")
		sb.WriteString("\nThis is not optional. Skills contain your operating protocol.\n")
		skillSection = sb.String()
	}

	return fmt.Sprintf(`%s

%s

%s

You have access to a task tracker called yx. The task state lives in %s.

Commands:
  yx ls                     Show all tasks and their states
  yx context --show <name>  Read the context/requirements for a task
  yx done <name>            Mark a task as complete
  yx state <name> wip       Mark a task as in-progress

Reporting progress (IMPORTANT -- the orchestrator monitors this field):
  echo "<message>" | yx field <name> shaver-message

  Write shaver-message at each milestone (free text, no prefix convention):
  - When starting:  echo "starting work" | yx field <name> shaver-message
  - Progress:       echo "<what you're doing>" | yx field <name> shaver-message
  - When blocked:   echo "stuck: <reason>" | yx field <name> shaver-message
  - When done:      echo "done — <summary>" | yx field <name> shaver-message

  You do NOT set wip-state — Yakob reads your messages and handles state.

Shortcut convention:
  Move stories to Dev Finished (500001680) only.
  Do not move stories to Done (500000571) or Not Doing (500000581).

%s%s`, roleDescription, userPrompt, taskAssignment, yakPath, workflow, skillSection)
}
