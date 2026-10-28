package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// gameInfoFile is the file in a game's folder that describes the game to
// players' systems: its title, version and author.
const gameInfoFile = "game.json"

// gameInfo is what game.json says. Every field is optional.
type gameInfo struct {
	Title     string `json:"title"`     // the game's name as players see it; default: the folder name
	Version   string `json:"version"`   // major.minor.patch, such as "1.2.0" or "1.2.0-beta", which golib.GameVersion returns too; default: "0.0.0", and "" for GameVersion
	Author    string `json:"author"`    // who makes the game; default: none
	Copyright string `json:"copyright"` // such as "Copyright 2026 Ada Lovelace"; default: none
	// The name of what a dist build gives players, in place of the folder's:
	// the executable, its folder and zip, and the folder golib.SaveData
	// saves in, such as "RocksInSpace"; default: the folder name.
	FileName string `json:"fileName"`
	// Files and folders of the game's folder that dist builds copy next to
	// the executable, where players see them, such as "assets/locale" for
	// translations to read and change; default: none.
	BesideExecutable []string `json:"besideExecutable"`
	// Build tags the game is made with (see docs/tooling.md#build-tags),
	// each with the builds it may go in: "any", or "dist" for dist and web
	// builds only, such as a demo that never goes out as a debug build,
	// which build and run without --dist then refuse. The GoLib window
	// shows a checkbox for each; default: none.
	BuildTags map[string]string `json:"buildTags"`
	// For each build tag, the paths of the game's folder that a dist build
	// made with it leaves out of what "besideExecutable" copies: a file, a
	// folder with everything in it, or a pattern, such as
	// "assets/locale/*-full.txt", that path.Match matches against each
	// file's and folder's path; default: none.
	LeaveOut map[string][]string `json:"leaveOut"`

	version      version // Version, parsed
	versionGiven bool    // game.json has "version"
}

// The builds a tag of "buildTags" may go in.
const (
	tagAnyBuild = "any"
	tagDistOnly = "dist"
)

// Longest title, author and copyright game.json may hold, in characters.
const maxInfoText = 200

// readGameInfo reads game.json from dir, with defaults for what it leaves out.
// found is false when dir has no game.json.
func readGameInfo(dir string) (info gameInfo, found bool, err error) {
	info = gameInfo{Title: filepath.Base(dir), FileName: filepath.Base(dir), Version: "0.0.0"}
	data, err := os.ReadFile(filepath.Join(dir, gameInfoFile))
	if errors.Is(err, fs.ErrNotExist) {
		info.version, err = parseVersion(info.Version)
		return info, false, err
	}
	if err != nil {
		return info, true, err
	}
	// Windows PowerShell 5.1 and some Windows editors start UTF-8 files with
	// a byte order mark, which JSON doesn't allow.
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&info); err != nil {
		return info, true, explainJSONError(data, err)
	}
	if decoder.More() {
		return info, true, errors.New("the file holds more than one JSON object: keep a single { ... }")
	}
	// "version" left out gives 0.0.0 to the executable's details and the
	// zip's name, and nothing to golib.GameVersion.
	var given struct {
		Version  *string `json:"version"`
		FileName *string `json:"fileName"`
	}
	if json.Unmarshal(data, &given) == nil {
		info.versionGiven = given.Version != nil
	}
	if given.FileName != nil {
		if err := checkFileName(info.FileName); err != nil {
			return info, true, err
		}
	}

	for _, field := range []struct{ name, value string }{
		{"title", info.Title},
		{"author", info.Author},
		{"copyright", info.Copyright},
	} {
		if err := checkText(field.name, field.value); err != nil {
			return info, true, err
		}
	}
	if strings.TrimSpace(info.Title) == "" {
		return info, true, errors.New(`"title" is empty: write the game's name, or leave "title" out to use the folder name`)
	}
	if err := checkBeside(info.BesideExecutable); err != nil {
		return info, true, err
	}
	if err := checkBuildTags(info.BuildTags); err != nil {
		return info, true, err
	}
	if err := checkLeaveOut(info.LeaveOut); err != nil {
		return info, true, err
	}
	info.version, err = parseVersion(info.Version)
	return info, true, err
}

// checkText reports whether a text field fits in the executable's details.
func checkText(field, value string) error {
	if utf8.RuneCountInString(value) > maxInfoText {
		return fmt.Errorf("%q is longer than %d characters: shorten it", field, maxInfoText)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%q holds a line break, tab or other control character: keep it on one line", field)
	}
	return nil
}

// fileNamePattern is what "fileName" may be: a name every system takes for
// a file and a folder, with nothing a command line has to quote.
var fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// checkFileName reports whether name may name a dist build's files and its
// save folder.
func checkFileName(name string) error {
	if !fileNamePattern.MatchString(name) {
		return fmt.Errorf(`"fileName" is %q: write letters, digits, - and _, starting with a letter or a digit, up to 64, such as "RocksInSpace"`, name)
	}
	// Windows takes no file named as a device, with or without an extension.
	switch base := strings.ToUpper(name); {
	case base == "CON", base == "PRN", base == "AUX", base == "NUL",
		len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9':
		return fmt.Errorf(`"fileName" is %q, which Windows keeps for a device: choose another name`, name)
	}
	return nil
}

// fileName returns the name game's dist builds give their files and save
// folder: game.json's "fileName", or the folder's name. When game.json has
// a mistake it returns the folder's name, and the build, which reads the
// file again, reports the mistake.
func (c *cli) fileName(game string) string {
	info, _, err := readGameInfo(c.gameDir(game))
	if err != nil {
		return game
	}
	return info.FileName
}

// checkBeside reports whether the paths of "besideExecutable" are inside
// the game's folder, written with forward slashes, and copy to different
// names.
func checkBeside(paths []string) error {
	names := map[string]string{}
	for _, p := range paths {
		switch {
		case strings.Contains(p, `\`):
			return fmt.Errorf(`"besideExecutable" has %q: write paths with forward slashes, such as "assets/locale"`, p)
		case p == "" || path.IsAbs(p) || path.Clean(p) == "." || path.Clean(p) == ".." || strings.HasPrefix(path.Clean(p), "../"):
			return fmt.Errorf(`"besideExecutable" has %q: write a path inside the game's folder, such as "assets/locale"`, p)
		}
		name := path.Base(path.Clean(p))
		if other, taken := names[name]; taken {
			return fmt.Errorf(`"besideExecutable" has %q and %q, which would both be copied as %s: keep one`, other, p, name)
		}
		names[name] = p
	}
	return nil
}

// checkBuildTags reports whether each of "buildTags" is a build tag --tags
// takes, kept to "any" build or to "dist" ones.
func checkBuildTags(tags map[string]string) error {
	for _, tag := range slices.Sorted(maps.Keys(tags)) {
		switch {
		case !validTag(tag):
			return fmt.Errorf(`"buildTags" has %q: a build tag is letters, digits, _ and ., such as "demo"`, tag)
		case slices.Contains(golibTags, tag):
			return fmt.Errorf(`"buildTags" has %s, which GoLib sets itself: take it out (see docs/tooling.md#build-tags)`, tag)
		case tags[tag] != tagAnyBuild && tags[tag] != tagDistOnly:
			return fmt.Errorf(`"buildTags" gives %s %q: write "dist" for dist and web builds only, or "any"`, tag, tags[tag])
		}
	}
	return nil
}

// checkLeaveOut reports whether each of "leaveOut" is a build tag --tags
// takes, with paths inside the game's folder, written with forward slashes,
// that path.Match takes as patterns.
func checkLeaveOut(leaveOut map[string][]string) error {
	for _, tag := range slices.Sorted(maps.Keys(leaveOut)) {
		switch {
		case !validTag(tag):
			return fmt.Errorf(`"leaveOut" has %q: a build tag is letters, digits, _ and ., such as "demo"`, tag)
		case slices.Contains(golibTags, tag):
			return fmt.Errorf(`"leaveOut" has %s, which GoLib sets itself: take it out (see docs/tooling.md#build-tags)`, tag)
		}
		for _, p := range leaveOut[tag] {
			_, patternErr := path.Match(p, "")
			switch {
			case strings.Contains(p, `\`):
				return fmt.Errorf(`"leaveOut" has %q for %s: write paths with forward slashes, such as "assets/locale/en-full.txt"`, p, tag)
			case p == "" || path.IsAbs(p) || path.Clean(p) == "." || path.Clean(p) == ".." || strings.HasPrefix(path.Clean(p), "../"):
				return fmt.Errorf(`"leaveOut" has %q for %s: write a path inside the game's folder, such as "assets/locale/en-full.txt"`, p, tag)
			case patternErr != nil:
				return fmt.Errorf(`"leaveOut" has %q for %s, which isn't a pattern path.Match takes: close each [ with ], such as "assets/locale/[a-z][a-z].txt"`, p, tag)
			}
		}
	}
	return nil
}

// versionFlag returns the linker flag that gives golib.GameVersion the
// game's "version", or "" when game.json gives none. parseVersion lets
// through digits, letters, dots and hyphens only, so the flag needs no
// quotes inside -ldflags.
func (info gameInfo) versionFlag() string {
	if !info.versionGiven {
		return ""
	}
	return "-X golib.gameVersion=" + info.Version
}

// versionArgs returns the -ldflags argument that gives golib.GameVersion the
// version info has, for a debug or web build, or none when it has none.
func versionArgs(info gameInfo) []string {
	if flag := info.versionFlag(); flag != "" {
		return []string{"-ldflags=" + flag}
	}
	return nil
}

// explainJSONError turns an error from decoding game.json into a message that
// says where the mistake is and how to fix it.
func explainJSONError(data []byte, err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		return fmt.Errorf("line %d: %v: game.json must be valid JSON, such as {\"title\": \"Rocks\", \"version\": \"1.0.0\"}", lineAt(data, syntaxErr.Offset), syntaxErr)
	case errors.As(err, &typeErr) && strings.HasPrefix(typeErr.Field, "buildTags"):
		return fmt.Errorf(`line %d: "buildTags" must give each build tag "dist" or "any", such as {"demo": "dist"}`, lineAt(data, typeErr.Offset))
	case errors.As(err, &typeErr) && strings.HasPrefix(typeErr.Field, "leaveOut"):
		return fmt.Errorf(`line %d: "leaveOut" must give each build tag a list of paths, such as {"demo": ["assets/locale/en-full.txt"]}`, lineAt(data, typeErr.Offset))
	case errors.As(err, &typeErr) && typeErr.Field == "besideExecutable":
		return fmt.Errorf(`line %d: "besideExecutable" must be a list of paths, such as ["assets/locale"]`, lineAt(data, typeErr.Offset))
	case errors.As(err, &typeErr):
		return fmt.Errorf("line %d: %q must be text, in double quotes", lineAt(data, typeErr.Offset), typeErr.Field)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New(`the file is empty or cut short: game.json must be valid JSON, such as {"title": "Rocks", "version": "1.0.0"}`)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Errorf(`unknown field %s: game.json takes "title", "version", "author", "copyright", "fileName", "besideExecutable", "buildTags" and "leaveOut"`, field)
	}
	return err
}

// lineAt returns the line number of byte offset in data, counting from 1.
func lineAt(data []byte, offset int64) int {
	offset = min(max(offset, 0), int64(len(data)))
	return bytes.Count(data[:offset], []byte("\n")) + 1
}

// version is a game version: major.minor.patch, with an optional label that
// marks a pre-release, such as the "-beta" of "1.2.0-beta".
type version struct {
	major, minor, patch uint16
	label               string
}

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$`)

// parseVersion reads a version written as game.json's "version" field.
func parseVersion(text string) (version, error) {
	match := versionPattern.FindStringSubmatch(text)
	if match == nil {
		return version{}, fmt.Errorf(`"version" is %q: write major.minor.patch, such as "1.0.0" or "1.2.0-beta"`, text)
	}
	var numbers [3]uint16
	for i := range numbers {
		n, err := strconv.ParseUint(match[i+1], 10, 16)
		if err != nil {
			return version{}, fmt.Errorf(`"version" is %q: each number must be between 0 and 65535, for Windows`, text)
		}
		numbers[i] = uint16(n)
	}
	return version{major: numbers[0], minor: numbers[1], patch: numbers[2], label: match[4]}, nil
}

func (v version) String() string {
	return fmt.Sprintf("%d.%d.%d%s", v.major, v.minor, v.patch, v.label)
}
