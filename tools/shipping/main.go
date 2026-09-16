// Shipping holds the parts of golib dist that are written in Go. The golib
// scripts build it into build/golib/ and run it; nobody needs to start it by
// hand.
//
//	shipping windows-resources -game <game folder> -arch <amd64|arm64> -out <file.syso>
//
// windows-resources reads the game's game.json and icon.png, both optional,
// and writes the Windows resources a shipped game carries: its icon, and the
// version information Explorer shows in the file's properties. The output is a
// .syso file, an object file that the Go linker adds to the program when it
// sits in the package's folder, so golib dist puts it next to the game's
// main.go while it builds, then deletes it.
//
// Every fact goes to standard output as one line, "ok <message>",
// "info <message>" or "fail <message>", which the scripts print as their own
// check lines. The exit code is 1 after a failure and 2 after a usage mistake.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const usage = "usage: shipping windows-resources -game <game folder> -arch <amd64|arm64> -out <file.syso>"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "windows-resources" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	flags := flag.NewFlagSet("windows-resources", flag.ContinueOnError)
	flags.SetOutput(stderr)
	gameDir := flags.String("game", "", "the game's folder")
	arch := flags.String("arch", "", "the processor the game is built for: amd64 or arm64")
	output := flags.String("out", "", "the .syso file to write")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() > 0 || *gameDir == "" || *arch == "" || *output == "" {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	report := func(level, message string) { fmt.Fprintln(stdout, level, message) }
	if err := windowsResources(*gameDir, *arch, *output, report); err != nil {
		report("fail", err.Error())
		return 1
	}
	return 0
}

// windowsResources writes the Windows resources of the game in gameDir, for
// arch processors, into the .syso file output. report receives the facts
// worth telling, as "ok" and "info" lines.
func windowsResources(gameDir, arch, output string, report func(level, message string)) error {
	target, found := targets[arch]
	if !found {
		return fmt.Errorf("GoLib can't make Windows resources for %q processors: use amd64 or arm64", arch)
	}
	name := filepath.Base(gameDir)
	shown := "games/" + name // how messages name the game's folder

	info, found, err := readGameInfo(gameDir)
	if err != nil {
		return fmt.Errorf("%s/%s: %w", shown, gameInfoFile, err)
	}
	details := fmt.Sprintf("%q, version %s", info.Title, info.Version)
	if info.Author != "" {
		details += ", by " + info.Author
	}
	if found {
		report("ok", fmt.Sprintf("%s/%s: the executable's details say %s", shown, gameInfoFile, details))
	} else {
		report("info", fmt.Sprintf("%s has no %s, so the executable's details say %s: add one to set the title, version and author (see docs/tooling.md)", shown, gameInfoFile, details))
	}
	resources := []resource{{kind: rtVersion, id: 1, data: versionInfo(info, name)}}

	icon, found, err := readIcon(filepath.Join(gameDir, iconFile))
	if err != nil {
		return fmt.Errorf("%s/%s: %w", shown, iconFile, err)
	}
	if found {
		resources = append(resources, iconResources(icon)...)
		size := icon.Bounds().Dx()
		report("ok", fmt.Sprintf("%s/%s (%d by %d pixels): the game's icon, in %d sizes from %d to %d pixels", shown, iconFile, size, size, len(iconSizes), iconSizes[0], iconSizes[len(iconSizes)-1]))
	} else {
		report("info", fmt.Sprintf("%s has no %s, so the game shows Windows' default icon: add a square PNG, ideally 256 by 256 pixels", shown, iconFile))
	}

	section, addressFields := resourceSection(resources)
	if err := os.WriteFile(output, objectFile(target, section, addressFields), 0o644); err != nil {
		return fmt.Errorf("cannot write the Windows resources: %w", err)
	}
	return nil
}
