package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// noticesFile is the file next to a dist build's executable that holds the
// licenses of the third-party software in the game.
const noticesFile = "THIRD-PARTY-LICENSES.txt"

// attributionFile, in a game's assets folder, says where the files that
// weren't made for the game come from, and under which license.
const attributionFile = "ATTRIBUTION.md"

// licenseFilePrefixes start the names of the files, in a module's folder,
// that hold its license and notices, in capitals.
var licenseFilePrefixes = []string{"LICENSE", "LICENCE", "COPYING", "COPYRIGHT", "NOTICE"}

// raylibNotices holds, for each raylib version, the notices of the libraries
// inside raylib whose licenses ask for them, which raylib's own LICENSE
// doesn't include: notices/raylib-<version>.txt. Nothing in raylib-go's
// module has them in a form that can be copied as it is, so they are
// written out here, and checked again for each new raylib version (see
// docs/tooling.md#third-party-licenses).
//
//go:embed notices/raylib-*.txt
var raylibNotices embed.FS

// notice is one piece of third-party software, or a group of files, in a
// game, with the files that hold its license.
type notice struct {
	title string // its name and version
	url   string // where to find it, or ""
	where string // where it is in the game
	files []string
	texts []licenseText // license texts that aren't in files, after them
}

// licenseText is a license text with a name for its heading.
type licenseText struct {
	name, text string
}

// raylibNotice returns the notices of the libraries inside raylib version,
// and false when GoLib has none for that version.
func raylibNotice(version string) (licenseText, bool) {
	text, err := raylibNotices.ReadFile("notices/raylib-" + version + ".txt")
	return licenseText{name: "libraries inside raylib " + version, text: string(text)}, err == nil
}

// moduleLicenseFiles returns the files in a module's folder that hold its
// license and notices, sorted by name.
func moduleLicenseFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var files []string
	for _, entry := range entries {
		name := strings.ToUpper(entry.Name())
		if entry.Type().IsRegular() && slices.ContainsFunc(licenseFilePrefixes, func(prefix string) bool {
			return strings.HasPrefix(name, prefix)
		}) {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

// thirdPartyNotices returns the text of THIRD-PARTY-LICENSES.txt for the game
// called title: a heading for each notice, followed by its license files.
func thirdPartyNotices(title string, notices []notice) ([]byte, error) {
	var text strings.Builder
	rule := strings.Repeat("=", 78)
	fmt.Fprintf(&text, "Third-party licenses for %s\n\n", title)
	fmt.Fprintf(&text, "%s includes the software and files below, made by others. Their\n", title)
	fmt.Fprintf(&text, "licenses ask for their notices to go with the game, so keep this file\n")
	fmt.Fprintf(&text, "with it.\n")
	for _, n := range notices {
		heading := []string{rule, n.title, n.url, n.where, rule}
		if n.url == "" {
			heading = slices.Delete(heading, 2, 3)
		}
		fmt.Fprintf(&text, "\n%s\n", strings.Join(heading, "\n"))
		texts := make([]licenseText, 0, len(n.files)+len(n.texts))
		for _, file := range n.files {
			content, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			texts = append(texts, licenseText{name: filepath.Base(file), text: string(content)})
		}
		for _, t := range append(texts, n.texts...) {
			body := strings.ReplaceAll(t.text, "\r\n", "\n")
			body = strings.TrimRight(strings.TrimLeft(body, "\n"), " \t\n")
			fmt.Fprintf(&text, "\n--- %s ---\n\n%s\n", t.name, body)
		}
	}
	return []byte(text.String()), nil
}
