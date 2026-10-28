//go:build !js

package device

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestLanguageModel loads the GGUF model GOLIB_TEST_MODEL names and asks it
// two questions, the second after the first answer, as a conversation goes.
// Without the variable, or without golibllm, it is skipped: models are big,
// and nobody has one by default.
func TestLanguageModel(t *testing.T) {
	path := os.Getenv("GOLIB_TEST_MODEL")
	if path == "" {
		t.Skip("set GOLIB_TEST_MODEL to a GGUF model to try language models")
	}
	if err := loadLLM(); err != nil {
		t.Skip(err)
	}
	start := time.Now()
	m, err := LoadLanguageModel(path, LanguageModelSettings{Context: 4096, GPULayers: 999})
	if err != nil {
		t.Fatal(err)
	}
	defer FreeLanguageModel(m)
	t.Logf("loaded in %v", time.Since(start))
	roles := []string{"system", "user"}
	texts := []string{"Eres una científica de 1993 atrapada en un ordenador. Respondes en español, en menos de 25 palabras.", "¿Quién eres?"}
	for turn := 0; turn < 2; turn++ {
		prompt, err := LanguageModelPrompt(m, roles, texts)
		if err != nil {
			t.Fatal(err)
		}
		start = time.Now()
		if err := StartLanguageModelReply(m, prompt, Sampling{Temperature: 0.7, TopP: 0.8, MinP: 0.05, RepeatPenalty: 1.1, Seed: 1, MaxTokens: 80}); err != nil {
			t.Fatal(err)
		}
		var reply strings.Builder
		first := time.Duration(0)
		pieces := 0
		for {
			piece, done, err := NextLanguageModelPiece(m)
			if err != nil {
				t.Fatal(err)
			}
			if done {
				break
			}
			if first == 0 {
				first = time.Since(start)
			}
			pieces++
			reply.WriteString(piece)
		}
		total := time.Since(start)
		t.Logf("turn %d: first piece after %v, %d pieces in %v (%.1f a second): %q", turn+1, first, pieces, total,
			float64(pieces)/(total-first).Seconds(), reply.String())
		if strings.TrimSpace(reply.String()) == "" {
			t.Fatal("the model answered nothing")
		}
		roles = append(roles, "assistant", "user")
		texts = append(texts, reply.String(), "¿Qué temperatura tienes ahora?")
	}
}
