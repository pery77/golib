package device

// The language models of the contract: llama.cpp, in golibllm, a native
// library of GoLib's own beside golibnative, called through purego on the
// desktop. Browsers have none. A model is loaded from a GGUF file and keeps
// one conversation's context; package golib runs it on goroutines of its
// own, away from the game loop, since a reply takes seconds.

// LanguageModel is a loaded model. Portable field: ID, 0 for none.
type LanguageModel struct {
	ID uintptr
}

// LanguageModelMarker is in the build of every game that loads a language
// model, and in no other: LoadLanguageModel keeps it in usesLanguageModel.
// golib looks for it to copy golibllm next to a game only when the game
// needs it. tools/cli has the same text.
const LanguageModelMarker = "golib-uses-language-models:llama.cpp"

// usesLanguageModel holds LanguageModelMarker once a model is loaded, which
// keeps the marker in the builds of the games that load one.
var usesLanguageModel string

// LanguageModelSettings says how to load a model.
type LanguageModelSettings struct {
	Context   int // tokens the conversation can take
	GPULayers int // layers on the graphics card: 0 none, a large number all
	Threads   int // processor threads, 0 for llama.cpp's choice
}

// Sampling says how a reply picks its words.
type Sampling struct {
	Temperature, TopP, MinP, RepeatPenalty float32
	Seed                                   uint32
	MaxTokens                              int
}
