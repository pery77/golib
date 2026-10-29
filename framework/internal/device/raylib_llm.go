//go:build !js

package device

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The language models on the desktop: golibllm, found next to the game, as
// golibnative is, or on the library search path, as golib test sets it. On
// Windows and Linux llama.cpp's backends are libraries beside it, which it
// loads from its own folder; on macOS they are inside it. It is loaded the
// first time a game loads a model.

// llmVersion is the version of golibllm's interface this file calls.
const llmVersion = 1

var llm struct {
	loaded bool
	err    error
	dir    string // golibllm's folder, where its backends are

	version, backends, load, free uintptr
	template, start, next         uintptr
}

// llmName is golibllm's file name on this system.
func llmName() string {
	switch runtime.GOOS {
	case "windows":
		return "golibllm.dll"
	case "darwin":
		return "golibllm.dylib"
	}
	return "golibllm.so"
}

// loadLLM loads golibllm and finds its functions, the first time.
func loadLLM() error {
	if llm.loaded {
		return llm.err
	}
	llm.loaded = true
	var places []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		places = append(places, filepath.Join(dir, llmName()), filepath.Join(dir, "..", "Frameworks", llmName()))
	}
	for _, dir := range filepath.SplitList(os.Getenv(libraryPathVariable())) {
		if dir != "" {
			places = append(places, filepath.Join(dir, llmName()))
		}
	}
	var handle uintptr
	var tried error
	for _, p := range places {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if h, err := openLibraryBeside(p); err == nil {
			handle = h
			llm.dir = filepath.Dir(p)
			break
		} else if tried == nil {
			tried = err
		}
	}
	if handle == 0 {
		why := "it isn't next to the game"
		if tried != nil {
			why = tried.Error()
		}
		llm.err = fmt.Errorf("language models need GoLib's library for them, %s: run golib setup, then build the game again with golib (%s)", llmName(), why)
		return llm.err
	}
	for _, f := range []struct {
		to   *uintptr
		name string
	}{
		{&llm.version, "golib_llm_version"}, {&llm.backends, "golib_llm_backends"},
		{&llm.load, "golib_llm_load"}, {&llm.free, "golib_llm_free"},
		{&llm.template, "golib_llm_template"}, {&llm.start, "golib_llm_start"},
		{&llm.next, "golib_llm_next"},
	} {
		p, err := librarySymbol(handle, f.name)
		if err != nil {
			llm.err = fmt.Errorf("%s has no %s: run golib setup for the library this GoLib needs", llmName(), f.name)
			return llm.err
		}
		*f.to = p
	}
	if v, _, _ := purego.SyscallN(llm.version); int32(v) != llmVersion {
		llm.err = fmt.Errorf("%s is version %d, and this GoLib needs version %d: run golib setup", llmName(), int32(v), llmVersion)
		return llm.err
	}
	dir := cText(llm.dir)
	purego.SyscallN(llm.backends, uintptr(unsafe.Pointer(&dir[0])))
	runtime.KeepAlive(dir)
	return nil
}

// libraryPathVariable is the variable that lists where the system looks for
// libraries, which golib test sets.
func libraryPathVariable() string {
	switch runtime.GOOS {
	case "windows":
		return "PATH"
	case "darwin":
		return "DYLD_LIBRARY_PATH"
	}
	return "LD_LIBRARY_PATH"
}

// cText is s with the zero byte C ends text with.
func cText(s string) []byte {
	return append([]byte(s), 0)
}

// goString reads the text C wrote into buf, up to its zero byte.
func goString(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// LoadLanguageModel reads a GGUF model, which takes seconds and a few GB of
// memory. Call it away from the game loop.
func LoadLanguageModel(path string, s LanguageModelSettings) (LanguageModel, error) {
	usesLanguageModel = LanguageModelMarker
	if err := loadLLM(); err != nil {
		return LanguageModel{}, err
	}
	p := cText(path)
	settings := [3]int32{int32(s.Context), int32(s.GPULayers), int32(s.Threads)}
	errBuf := make([]byte, 512)
	m, _, _ := purego.SyscallN(llm.load, uintptr(unsafe.Pointer(&p[0])), uintptr(unsafe.Pointer(&settings[0])),
		uintptr(unsafe.Pointer(&errBuf[0])), uintptr(len(errBuf)))
	runtime.KeepAlive(p)
	runtime.KeepAlive(&settings)
	if m == 0 {
		return LanguageModel{}, errors.New(goString(errBuf))
	}
	return LanguageModel{ID: m}, nil
}

// FreeLanguageModel frees a model and its memory.
func FreeLanguageModel(m LanguageModel) {
	if m.ID != 0 {
		purego.SyscallN(llm.free, m.ID)
	}
}

// LanguageModelPrompt writes a conversation, its roles and texts, in the
// model's own chat format, ready for the assistant's reply.
func LanguageModelPrompt(m LanguageModel, roles, texts []string) (string, error) {
	var keep [][]byte
	pointers := func(list []string) []uintptr {
		out := make([]uintptr, len(list)+1)
		for i, s := range list {
			b := cText(s)
			keep = append(keep, b)
			out[i] = uintptr(unsafe.Pointer(&b[0]))
		}
		return out
	}
	r, t := pointers(roles), pointers(texts)
	size := 4096
	for _, s := range texts {
		size += 2 * len(s)
	}
	for try := 0; try < 2; try++ {
		out := make([]byte, size)
		n, _, _ := purego.SyscallN(llm.template, m.ID, uintptr(unsafe.Pointer(&r[0])), uintptr(unsafe.Pointer(&t[0])),
			uintptr(len(roles)), uintptr(unsafe.Pointer(&out[0])), uintptr(len(out)))
		runtime.KeepAlive(keep)
		runtime.KeepAlive(r)
		runtime.KeepAlive(t)
		if int32(n) < 0 {
			return "", errors.New("llama.cpp doesn't know this model's chat format")
		}
		if int(int32(n)) < len(out) {
			return string(out[:int32(n)]), nil
		}
		size = int(int32(n)) + 1
	}
	return "", errors.New("the conversation didn't fit in the prompt's buffer")
}

// StartLanguageModelReply begins the model's reply to prompt.
func StartLanguageModelReply(m LanguageModel, prompt string, s Sampling) error {
	p := cText(prompt)
	sampling := [6]float32{s.Temperature, s.TopP, s.MinP, s.RepeatPenalty, float32(s.Seed % (1 << 24)), float32(s.MaxTokens)}
	errBuf := make([]byte, 512)
	r, _, _ := purego.SyscallN(llm.start, m.ID, uintptr(unsafe.Pointer(&p[0])), uintptr(unsafe.Pointer(&sampling[0])),
		uintptr(unsafe.Pointer(&errBuf[0])), uintptr(len(errBuf)))
	runtime.KeepAlive(p)
	runtime.KeepAlive(&sampling)
	if int32(r) != 0 {
		return errors.New(goString(errBuf))
	}
	return nil
}

// NextLanguageModelPiece writes the reply's next piece of text, which can
// be empty, and says whether the reply is done.
func NextLanguageModelPiece(m LanguageModel) (string, bool, error) {
	buf := make([]byte, 1024)
	n, _, _ := purego.SyscallN(llm.next, m.ID, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	switch k := int32(n); {
	case k < 0:
		return "", true, errors.New("llama.cpp failed while writing the reply")
	case k == 0:
		return "", true, nil
	default:
		return string(buf[:k]), false, nil
	}
}
