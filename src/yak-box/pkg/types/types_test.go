package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkerNames(t *testing.T) {
	assert.Len(t, WorkerNames, 4)
	assert.Contains(t, WorkerNames, "Yakriel")
	assert.Contains(t, WorkerNames, "Yakueline")
	assert.Contains(t, WorkerNames, "Yakov")
	assert.Contains(t, WorkerNames, "Yakira")
}


func TestWorkerCreation(t *testing.T) {
	now := time.Now()
	worker := Worker{
		Name:          "test-worker",
		DisplayName:   "Test Worker",
		ContainerName: "yakbox-test-worker",
		Runtime:       "sandboxed",
		CWD:           "/tmp/test",
		YakPath:       ".yaks",
		Tasks:         []string{"task1", "task2"},
		SpawnedAt:     now,
	}

	assert.Equal(t, "test-worker", worker.Name)
	assert.Equal(t, "Test Worker", worker.DisplayName)
	assert.Equal(t, "yakbox-test-worker", worker.ContainerName)
	assert.Equal(t, "sandboxed", worker.Runtime)
	assert.Equal(t, "/tmp/test", worker.CWD)
	assert.Equal(t, ".yaks", worker.YakPath)
	assert.Len(t, worker.Tasks, 2)
	assert.Equal(t, now, worker.SpawnedAt)
}

func TestResourceProfileCreation(t *testing.T) {
	tmpfs := map[string]string{
		"/tmp": "1G",
	}

	profile := ResourceProfile{
		Name:   "default",
		CPUs:   "2",
		Memory: "4G",
		PIDs:   1024,
		Tmpfs:  tmpfs,
	}

	assert.Equal(t, "default", profile.Name)
	assert.Equal(t, "2", profile.CPUs)
	assert.Equal(t, "4G", profile.Memory)
	assert.Equal(t, 1024, profile.PIDs)
	assert.Equal(t, tmpfs, profile.Tmpfs)
	assert.Equal(t, "1G", profile.Tmpfs["/tmp"])
}
