package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
	Version   string `json:"version"`   // major.minor.patch, such as "1.2.0" or "1.2.0-beta"; default: "0.0.0"
	Author    string `json:"author"`    // who makes the game; default: none
	Copyright string `json:"copyright"` // such as "Copyright 2026 Ada Lovelace"; default: none

	version version // Version, parsed
}

// Longest title, author and copyright game.json may hold, in characters.
const maxInfoText = 200

// readGameInfo reads game.json from dir, with defaults for what it leaves out.
// found is false when dir has no game.json.
func readGameInfo(dir string) (info gameInfo, found bool, err error) {
	info = gameInfo{Title: filepath.Base(dir), Version: "0.0.0"}
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

// explainJSONError turns an error from decoding game.json into a message that
// says where the mistake is and how to fix it.
func explainJSONError(data []byte, err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		return fmt.Errorf("line %d: %v: game.json must be valid JSON, such as {\"title\": \"Rocks\", \"version\": \"1.0.0\"}", lineAt(data, syntaxErr.Offset), syntaxErr)
	case errors.As(err, &typeErr):
		return fmt.Errorf("line %d: %q must be text, in double quotes", lineAt(data, typeErr.Offset), typeErr.Field)
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New(`the file is empty or cut short: game.json must be valid JSON, such as {"title": "Rocks", "version": "1.0.0"}`)
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return fmt.Errorf(`unknown field %s: game.json takes "title", "version", "author" and "copyright"`, field)
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
