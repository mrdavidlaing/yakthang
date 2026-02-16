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
}

type ResourceProfile struct {
	Name   string
	CPUs   string
	Memory string
	Swap   string
	PIDs   int
	Tmpfs  map[string]string
}
