// Package types defines shared data structures for yak-box.
package types

import "time"

type Persona struct {
	Name        string
	Emoji       string
	Trait       string
	Personality string
}

type Worker struct {
	Name          string
	DisplayName   string
	ContainerName string
	Runtime       string
	CWD           string
	YakPath       string
	Tasks         []string
	SpawnedAt     time.Time
	SessionName   string
	WorktreePath  string // Path to git worktree (if using --auto-worktree)
	PidFile       string // Path to PID file for native workers
}

type ResourceProfile struct {
	Name   string
	CPUs   string
	Memory string
	Swap   string
	PIDs   int
	Tmpfs  map[string]string
}
