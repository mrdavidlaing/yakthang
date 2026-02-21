// Package prompt handles prompt generation and rendering for yak-box.
package prompt

import (
	"fmt"
)

// BuildPrompt assembles the initial prompt for a worker
func BuildPrompt(mode string, yakPath string, userPrompt string, tasks []string) string {
	var roleDescription string
	if mode == "plan" {
		roleDescription = "Your supervisor is Yakob. The yaks are tasks — your job is to scout them and plan the shave. Do NOT pick up the clippers."
	} else {
		roleDescription = "Your supervisor is Yakob. The yaks are tasks — your job is to shave them clean."
	}

	var workflow string
	if mode == "plan" {
		workflow = `Workflow:
1. Run 'yx ls' to see available tasks
2. Pick a task, read its context with 'yx context --show <name>'
3. Set it to wip: 'yx state <name> wip'
4. Report status: echo "wip: starting plan" | yx field <name> agent-status
5. Analyze the codebase, understand the problem, and write a detailed plan
6. Save the plan where it makes sense (e.g. a markdown file, or in yx context)
7. Report: echo "blocked: plan ready for review" | yx field <name> agent-status
8. STOP and wait — do NOT implement. Your job is to plan, not build.

Focus on the tasks assigned to you. Do not modify tasks outside your scope.`
	} else {
		workflow = `Workflow:
1. Run 'yx ls' to see available tasks
2. Pick a task, read its context with 'yx context --show <name>'
3. Set it to wip: 'yx state <name> wip'
4. Report status: echo "wip: starting" | yx field <name> agent-status
5. Do the work (update agent-status as you make progress)
6. When done: 'yx done <name>' then echo "done: <summary>" | yx field <name> agent-status
7. If blocked: echo "blocked: <reason>" | yx field <name> agent-status

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

	return fmt.Sprintf(`%s

%s

%s

You have access to a task tracker called yx. The task state lives in %s.

Commands:
  yx ls                     Show all tasks and their states
  yx context --show <name>  Read the context/requirements for a task
  yx done <name>            Mark a task as complete
  yx state <name> wip       Mark a task as in-progress

Reporting status (IMPORTANT -- the orchestrator monitors these fields):
  echo "<status>" | yx field <name> agent-status

  Write agent-status at each transition:
  - When starting:  echo "wip: starting" | yx field <name> agent-status
  - Progress:       echo "wip: <what you're doing>" | yx field <name> agent-status
  - When blocked:   echo "blocked: <reason>" | yx field <name> agent-status
  - When done:      echo "done: <summary>" | yx field <name> agent-status

%s`, roleDescription, userPrompt, taskAssignment, yakPath, workflow)
}
