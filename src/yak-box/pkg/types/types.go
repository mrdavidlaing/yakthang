// Package types defines shared data structures for yak-box.
package types

import "time"

// WorkerNames is the pool of available worker names.
// These are simple identifiers used for display and home directory isolation.
var WorkerNames = []string{"Yakriel", "Yakueline", "Yakov", "Yakira"}

type Worker struct {
	Name          string
	WorkerName    string // Yak-shaver identity (e.g. "Yakriel")
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
