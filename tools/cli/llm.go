package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The language models of the framework (framework/languagemodel.go) run on
// llama.cpp, in golibllm, a native library of GoLib's own built from
// native/llm/ by .github/workflows/llm.yml and published as a release, a zip
// per platform: on Windows and Linux golibllm and llama.cpp's backends, each
// a library, and on macOS golibllm alone. golib setup unpacks it into
// .tools/llm/, checking the zip against the checksum written here, and the
// builds of a game that loads a model carry every file of it. A game's
// models, GGUF files too big for git, are listed in its game.json's
// "models", and golib downloads each into the game's models/ folder the
// first time a build needs it.

// llmMarker is in the build of every game that loads a language model, and
// in no other: framework/internal/device/llm.go's LanguageModelMarker.
const llmMarker = "golib-uses-language-models:llama.cpp"

// llmRelease names the GitHub release, in nativeRepository, that setup
// downloads golibllm from: golibllm-<os>-<arch>.zip.
const llmRelease = "llm-2"

// llmChecksums holds the SHA-256 of each zip of llmRelease, written here when
// the release is made (see native/README.md). A platform without one has no
// release yet: setup leaves a library built by hand with native/llm/build.sh
// in place, and says so.
var llmChecksums = map[string]string{}

// llamaLicense holds llama.cpp's license, for THIRD-PARTY-LICENSES.txt.
//
//go:embed notices/llama-cpp.txt
var llamaLicense embed.FS

// llamaNotice is llama.cpp's license, for THIRD-PARTY-LICENSES.txt.
func llamaNotice(where string) notice {
	text, err := llamaLicense.ReadFile("notices/llama-cpp.txt")
	if err != nil {
		panic(errors.Join(errors.New("tools/cli/notices/llama-cpp.txt is missing"), err))
	}
	title, body, _ := strings.Cut(string(text), "\n")
	return notice{title: "llama.cpp b11972, with ggml", url: "https://github.com/ggml-org/llama.cpp", where: where,
		texts: []licenseText{{name: title, text: body}}}
}

// llmName is golibllm's file name on the platform the CLI builds for.
func (c *cli) llmName() string {
	switch c.goos {
	case "windows":
		return "golibllm.dll"
	case "darwin":
		return "golibllm.dylib"
	}
	return "golibllm.so"
}

// llmDir is where golib setup puts golibllm and its backends: .tools/llm/.
func (c *cli) llmDir() string {
	return c.path(".tools", "llm")
}

// syncLLM makes sure .tools/llm/ holds golibllm for this platform,
// downloading and unpacking its release when it is missing or different.
func (c *cli) syncLLM() {
	platform := c.goos + "/" + c.goarch
	dir := c.llmDir()
	sum, found := c.llms[platform]
	if !found {
		if isFile(filepath.Join(dir, c.llmName())) {
			c.check("ok", fmt.Sprintf("GoLib's library for language models, built by hand in .tools/llm/ (%s has no release for %s yet)", llmRelease, platform))
		} else {
			c.check("info", fmt.Sprintf("GoLib's library for language models has no release for %s yet, so games with language models won't build here: build it with native/llm/build.sh (see native/README.md)", platform))
		}
		return
	}
	stamp := filepath.Join(dir, "release.sha256")
	if data, err := os.ReadFile(stamp); err == nil && strings.TrimSpace(string(data)) == sum && isFile(filepath.Join(dir, c.llmName())) {
		c.check("ok", fmt.Sprintf("GoLib's library for language models %s in .tools/llm/", llmRelease))
		return
	}
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/golibllm-%s-%s.zip", nativeRepository, llmRelease, c.goos, c.goarch)
	data, err := c.download(url)
	if err == nil && sha256Hex(data) != sum {
		err = fmt.Errorf("%s isn't the file GoLib expects: its SHA-256 is %s, not %s", url, sha256Hex(data), sum)
	}
	if err == nil {
		err = unzipInto(data, dir)
	}
	if err == nil {
		err = os.WriteFile(stamp, []byte(sum+"\n"), 0o644)
	}
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot download GoLib's library for language models %s: %v", llmRelease, err))
		return
	}
	c.check("ok", fmt.Sprintf("GoLib's library for language models %s downloaded into .tools/llm/", llmRelease))
}

// unzipInto empties dir and puts the files of a zip in it, flat.
func unzipInto(data []byte, dir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		in, err := f.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(in)
		in.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), body, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// llmFiles are the files of .tools/llm/ a game carries: the libraries.
func (c *cli) llmFiles() []string {
	entries, err := os.ReadDir(c.llmDir())
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		// Linux's libraries are named libllama.so.0 and the like.
		library := strings.HasSuffix(name, ".dll") || strings.HasSuffix(name, ".dylib") || strings.Contains(name, ".so")
		if e.IsDir() || !library {
			continue
		}
		files = append(files, filepath.Join(c.llmDir(), name))
	}
	sort.Strings(files)
	return files
}

// usesLanguageModels says whether the program built at path loads a
// language model, and so needs golibllm.
func usesLanguageModels(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return bytes.Contains(data, []byte(llmMarker)), nil
}

// copyLLM copies golibllm and its backends into dir when the game built at
// exe loads a language model, and returns what it copied.
func (c *cli) copyLLM(game, exe, dir string) ([]string, error) {
	uses, err := usesLanguageModels(exe)
	if err != nil || !uses {
		return nil, err
	}
	if !isFile(filepath.Join(c.llmDir(), c.llmName())) {
		return nil, fmt.Errorf("%s loads a language model, which needs GoLib's library for them, and .tools/llm/ has no %s: run golib setup (see native/README.md)", c.gameModule(game), c.llmName())
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var copied []string
	for _, f := range c.llmFiles() {
		target := filepath.Join(dir, filepath.Base(f))
		if err := syncFile(f, target); err != nil {
			return nil, err
		}
		copied = append(copied, target)
	}
	return copied, nil
}

// modelSource is where game.json's "models" says a model comes from.
type modelSource struct {
	URL    string `json:"url"`    // where to download it, https
	SHA256 string `json:"sha256"` // its checksum, which the download must have
}

// syncModels downloads the models game.json lists that the game's models/
// folder hasn't got, checking each, and reports each it downloads.
func (c *cli) syncModels(game string, info gameInfo) bool {
	names := make([]string, 0, len(info.Models))
	for name := range info.Models {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := c.gameDir(game, modelsFolder, name)
		if isFile(target) {
			continue
		}
		src := info.Models[name]
		c.check("info", fmt.Sprintf("downloading the language model %s for %s from %s, once: it is big, so this takes a while", name, c.gameModule(game), src.URL))
		if err := downloadFile(src.URL, src.SHA256, target); err != nil {
			c.check("fail", fmt.Sprintf("cannot download the language model %s: %v", name, err))
			return false
		}
		c.check("ok", fmt.Sprintf("downloaded the language model %s into %s", name, c.shown(target)))
	}
	return true
}

// modelsFolder is the folder of a game that holds its language models.
const modelsFolder = "models"

// downloadFile downloads url into target, a piece at a time, so a file of
// gigabytes never sits in memory, and keeps it only if its SHA-256 is sum.
func downloadFile(url, sum, target string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", url, response.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	partial := target + ".part"
	out, err := os.Create(partial)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, hash), response.Body)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		if got := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(got, sum) {
			err = fmt.Errorf("%s isn't the file game.json names: its SHA-256 is %s, not %s", url, got, sum)
		}
	}
	if err != nil {
		os.Remove(partial)
		return err
	}
	return os.Rename(partial, target)
}

// syncBigFile copies source to target unless target is as big and no older:
// a model is too big to compare byte by byte, as syncFile does.
func syncBigFile(source, target string) error {
	from, err := os.Stat(source)
	if err != nil {
		return err
	}
	if to, err := os.Stat(target); err == nil && to.Size() == from.Size() && !to.ModTime().Before(from.ModTime()) {
		return nil
	}
	return copyFile(source, target)
}

// copyModels copies the game's models into dir, for a dist build, and
// returns their names.
func (c *cli) copyModels(game, dir string) ([]string, error) {
	entries, err := os.ReadDir(c.gameDir(game, modelsFolder))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".gguf") {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := syncBigFile(c.gameDir(game, modelsFolder, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return nil, err
		}
		names = append(names, e.Name())
	}
	return names, nil
}
