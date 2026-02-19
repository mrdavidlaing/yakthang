// Package persona manages persona data for yak-box workers and sessions.
package persona

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/yakthang/yakbox/internal/workspace"
	"github.com/yakthang/yakbox/pkg/types"
)

// Persona definitions with their traits
var personas = []types.Persona{
	{
		Name:        "Yakriel",
		Emoji:       "🦬🪒",
		Trait:       "Precise and methodical. Measures twice, shaves once.",
		Personality: "",
	},
	{
		Name:        "Yakueline",
		Emoji:       "🦬💈",
		Trait:       "Fast and fearless. Ships first, asks forgiveness later.",
		Personality: "",
	},
	{
		Name:        "Yakov",
		Emoji:       "🦬🔔",
		Trait:       "Cautious and thorough. Better safe than shorn.",
		Personality: "",
	},
	{
		Name:        "Yakira",
		Emoji:       "🦬🧶",
		Trait:       "Cheerful and communicative. Leaves detailed status updates.",
		Personality: "",
	},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GetRandomPersona returns a random persona for a worker
func GetRandomPersona() types.Persona {
	index := rand.Intn(len(personas))
	p := personas[index]

	// Load personality from file
	p.Personality = LoadPersonality(p.Name)

	return p
}

// LoadPersonality loads personality text for a given persona name
func LoadPersonality(name string) string {
	// Try to load from standard location
	workspaceRoot, err := workspace.FindRoot()
	if err != nil {
		return getDefaultPersonality(name)
	}

	personalityFile := filepath.Join(workspaceRoot, ".opencode", "personalities", fmt.Sprintf("%s-worker.md", name))

	content, err := os.ReadFile(personalityFile)
	if err != nil {
		return getDefaultPersonality(name)
	}

	return string(content)
}

// getDefaultPersonality returns a default personality if file not found
func getDefaultPersonality(name string) string {
	switch name {
	case "Yakriel":
		return "You are precise and methodical. You measure twice and shave once."
	case "Yakueline":
		return "You are fast and fearless. You ship first and ask forgiveness later."
	case "Yakov":
		return "You are cautious and thorough. You believe better safe than shorn."
	case "Yakira":
		return "You are cheerful and communicative. You leave detailed status updates."
	default:
		return "You are a helpful assistant."
	}
}
