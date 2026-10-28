package golib

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golib/internal/device"
)

// modelsDir is the folder, inside the game's folder, that holds its
// language models: GGUF files, too big to go inside the executable.
const modelsDir = "models"

// LanguageModel is a language model that runs inside the game, on the
// player's computer, with nothing to install and no connection: llama.cpp,
// in GoLib's library golibllm, reading a GGUF file from the game's models
// folder. Make one with NewLanguageModel and keep it; it loads in the
// background the first time it is asked for a reply, or when Load is
// called, which takes seconds. Desktop only: in a browser, Err says so.
type LanguageModel struct {
	name    string
	options LanguageModelOptions

	mu     sync.Mutex
	state  modelState
	err    error
	model  device.LanguageModel
	turn   sync.Mutex // one reply at a time: the model has one context
	loaded chan struct{}
}

type modelState int

const (
	modelIdle modelState = iota
	modelLoading
	modelReady
	modelFailed
)

// LanguageModelOptions changes how NewLanguageModel loads a model.
type LanguageModelOptions struct {
	// Context is how many tokens the conversation can hold: the system
	// prompt, the messages and the reply. Default: 4096. More takes more
	// memory.
	Context int
	// CPUOnly keeps the model off the graphics card, which it uses where
	// llama.cpp has a backend for it: Metal on Apple silicon, Vulkan on
	// Windows and Linux.
	CPUOnly bool
	// Threads is how many processor threads it uses. Default: llama.cpp's
	// choice, about one per core.
	Threads int
}

// NewLanguageModel returns the model in the GGUF file name, in the game's
// models folder, games/<game>/models/, with forward slashes. Making one
// reads nothing. Pass at most one LanguageModelOptions.
func NewLanguageModel(name string, options ...LanguageModelOptions) *LanguageModel {
	m := &LanguageModel{name: name, loaded: make(chan struct{})}
	if len(options) > 0 {
		m.options = options[0]
	}
	if m.options.Context <= 0 {
		m.options.Context = 4096
	}
	return m
}

// Load starts loading the model in the background, if it hasn't started,
// so the first reply doesn't wait for it: call it when the game starts.
func (m *LanguageModel) Load() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != modelIdle {
		return
	}
	m.state = modelLoading
	go m.load()
}

func (m *LanguageModel) load() {
	path, err := modelPath(m.name)
	var model device.LanguageModel
	if err == nil {
		gpu := 999
		if m.options.CPUOnly {
			gpu = 0
		}
		model, err = device.LoadLanguageModel(path, device.LanguageModelSettings{Context: m.options.Context, GPULayers: gpu, Threads: m.options.Threads})
	}
	m.mu.Lock()
	if err != nil {
		m.state, m.err = modelFailed, fmt.Errorf("golib.NewLanguageModel(%q): %w", m.name, err)
	} else {
		m.state, m.model = modelReady, model
		runtime.AddCleanup(m, device.FreeLanguageModel, model)
	}
	m.mu.Unlock()
	close(m.loaded)
}

// Ready reports whether the model is loaded.
func (m *LanguageModel) Ready() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == modelReady
}

// Err says why the model couldn't load, or nil while it loads, once it has
// loaded, and before it is asked to.
func (m *LanguageModel) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.err
}

// modelPath finds a model's file: in the game's models folder in a debug
// build; beside the executable, or in the app's Resources on macOS, in a
// golib dist build.
func modelPath(name string) (string, error) {
	var places []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if distBuild {
			places = append(places, filepath.Join(dir, modelsDir, name), filepath.Join(dir, "..", "Resources", modelsDir, name))
		} else if workDir, err := os.Getwd(); err == nil {
			places = append(places, filepath.Join(debugGameDir(workDir, exe, isDir), modelsDir, name))
		}
	}
	for _, p := range places {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	if len(places) == 0 {
		return "", errors.New("can't tell where the game is")
	}
	return "", fmt.Errorf("there is no %s: put the GGUF file in the game's %s folder, or list it in game.json's \"models\" for golib to download it", places[0], modelsDir)
}

// The roles of a conversation's messages.
const (
	RoleSystem    = "system"    // how the model is to behave: who it is, what it knows
	RoleUser      = "user"      // what the player says
	RoleAssistant = "assistant" // what the model said
)

// ChatMessage is one message of a conversation: who says it and what.
type ChatMessage struct {
	Role string // RoleSystem, RoleUser or RoleAssistant
	Text string
}

// ReplyOptions changes how a reply is written. Fields left at zero get
// their default.
type ReplyOptions struct {
	// MaxTokens is the longest the reply can be, in tokens, about three
	// quarters of a word each. Default: 256.
	MaxTokens int
	// Temperature is how adventurous the words are: 0 always picks the
	// likeliest, 1 is varied. Default: 0.7. Set Greedy for 0.
	Temperature float32
	// Greedy always picks the likeliest word, so the same conversation gets
	// the same reply.
	Greedy bool
	// TopP and MinP leave unlikely words out. Defaults: 0.8 and 0.05.
	TopP, MinP float32
	// RepeatPenalty makes the model repeat itself less. Default: 1.1; 1 is
	// none.
	RepeatPenalty float32
	// Seed starts the random choices. Default: a new one for each reply,
	// from RandomInt.
	Seed uint32
}

// Reply is the model's answer, written in the background, a piece at a
// time: read Text in Update to show it as it comes.
type Reply struct {
	mu   sync.Mutex
	text strings.Builder
	done bool
	err  error
	stop bool
}

// Text is what the reply says so far.
func (r *Reply) Text() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text.String()
}

// Done reports whether the reply is finished: written, stopped or failed.
func (r *Reply) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

// Err says why the reply failed, or nil.
func (r *Reply) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Stop ends the reply where it is: Text keeps what it has. A reply waiting
// for its turn never starts.
func (r *Reply) Stop() {
	r.mu.Lock()
	r.stop = true
	r.mu.Unlock()
}

func (r *Reply) stopped() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stop
}

func (r *Reply) finish(err error) {
	r.mu.Lock()
	r.done, r.err = true, err
	r.mu.Unlock()
}

// Reply starts the model's answer to a conversation, from its first
// message, usually RoleSystem, to its last, usually the player's RoleUser
// message, and returns at once: the model loads first if it hasn't, and
// replies one at a time, in the order they were asked for. Each reply reads
// the whole conversation, but the part the model read last time, from its
// start, isn't read again, so keep the messages in order and add to the
// end. Pass at most one ReplyOptions.
func (m *LanguageModel) Reply(messages []ChatMessage, options ...ReplyOptions) *Reply {
	var o ReplyOptions
	if len(options) > 0 {
		o = options[0]
	}
	if o.MaxTokens <= 0 {
		o.MaxTokens = 256
	}
	if o.Temperature <= 0 {
		o.Temperature = 0.7
	}
	if o.Greedy {
		o.Temperature = 0
	}
	if o.TopP <= 0 {
		o.TopP = 0.8
	}
	if o.MinP <= 0 {
		o.MinP = 0.05
	}
	if o.RepeatPenalty <= 0 {
		o.RepeatPenalty = 1.1
	}
	if o.Seed == 0 {
		o.Seed = uint32(RandomInt(1, 1<<24-1))
	}
	roles := make([]string, len(messages))
	texts := make([]string, len(messages))
	for i, msg := range messages {
		roles[i], texts[i] = msg.Role, msg.Text
	}
	r := &Reply{}
	m.Load()
	go m.write(r, roles, texts, o)
	return r
}

func (m *LanguageModel) write(r *Reply, roles, texts []string, o ReplyOptions) {
	<-m.loaded
	m.turn.Lock()
	defer m.turn.Unlock()
	if err := m.Err(); err != nil {
		r.finish(err)
		return
	}
	if r.stopped() {
		r.finish(nil)
		return
	}
	prompt, err := device.LanguageModelPrompt(m.model, roles, texts)
	if err == nil {
		err = device.StartLanguageModelReply(m.model, prompt, device.Sampling{Temperature: o.Temperature, TopP: o.TopP,
			MinP: o.MinP, RepeatPenalty: o.RepeatPenalty, Seed: o.Seed, MaxTokens: o.MaxTokens})
	}
	if err != nil {
		r.finish(fmt.Errorf("golib.LanguageModel.Reply: %w", err))
		return
	}
	for {
		piece, done, err := device.NextLanguageModelPiece(m.model)
		if err != nil {
			r.finish(fmt.Errorf("golib.LanguageModel.Reply: %w", err))
			return
		}
		if done || r.stopped() {
			break
		}
		r.mu.Lock()
		r.text.WriteString(piece)
		r.mu.Unlock()
	}
	r.finish(nil)
}
