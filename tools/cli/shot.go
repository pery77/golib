package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Shot settings: the frame captured when none is given, one second of game
// time, how long a game may run before it is stopped, and the largest --scale.
const (
	shotDefaultFrame = 60
	shotTimeout      = 120 * time.Second
	shotMaxScale     = 8
)

// shot runs a debug build of a game in a hidden window and saves screenshots
// of chosen frames (see docs/tooling.md#screenshots). Its options are frame
// numbers, --input, --save, --scale and a game name, in any order.
func (c *cli) shot(options []string) int {
	var names []string
	var frames []int
	input, save := "", ""
	scale := 1
	web := false
	for i := 0; i < len(options); i++ {
		option := options[i]
		value := ""
		switch {
		case option == "":
			return c.usage("shot got an empty argument")
		case option == "--web":
			web = true
			continue
		case option == "--input", option == "--save", option == "--scale":
			if i+1 >= len(options) {
				return c.usage(shotOptionUsage(option))
			}
			i++
			value = options[i]
		case strings.HasPrefix(option, "--input="), strings.HasPrefix(option, "--save="), strings.HasPrefix(option, "--scale="):
			option, value, _ = strings.Cut(option, "=")
		case strings.HasPrefix(option, "-"):
			return c.usage("unknown option for shot: " + option)
		case strings.Trim(option, "0123456789") == "":
			frame, _ := strconv.Atoi(option)
			if len(option) > 6 || frame < 1 {
				return c.usage(fmt.Sprintf("frame numbers go from 1 to 999999 (got: %s)", option))
			}
			frames = append(frames, frame)
			continue
		default:
			names = append(names, option)
			continue
		}
		switch option {
		case "--input":
			input = value
		case "--save":
			if value == "" {
				return c.usage(shotOptionUsage(option))
			}
			save = value
		case "--scale":
			size, err := strconv.Atoi(value)
			if err != nil || size < 1 || size > shotMaxScale {
				return c.usage(fmt.Sprintf("shot --scale takes a whole number from 1 to %d, which the screenshots are enlarged by (got: %s)", shotMaxScale, value))
			}
			scale = size
		}
	}
	savePath := ""
	if save != "" {
		path, err := filepath.Abs(save)
		if err != nil {
			return c.usage(fmt.Sprintf("shot --save cannot use the path %s: %v", save, err))
		}
		savePath = path
	}
	if len(frames) == 0 {
		frames = []int{shotDefaultFrame}
	}
	if savePath != "" && !isFile(savePath) {
		c.check("fail", fmt.Sprintf("shot --save: there is no file %s (it holds the data the game starts with, such as {\"progress\": {\"level\": 8}})", c.shown(savePath)))
		return c.summary("shot")
	}
	slices.Sort(frames)
	frames = slices.Compact(frames)
	game, exitCode := c.resolveGame("shot", names)
	if game == "" {
		return exitCode
	}
	if web {
		if savePath != "" {
			return c.usage("shot --web cannot use --save yet: a page cannot read a file from this machine")
		}
		return c.shotWeb(game, frames, input, scale)
	}
	exe := c.buildGame(game)
	if exe == "" {
		return c.summary("shot")
	}

	shots := c.path("build", game, "shots")
	err := os.RemoveAll(shots)
	if err == nil {
		err = os.MkdirAll(shots, 0o755)
	}
	if err != nil {
		c.check("fail", fmt.Sprintf("cannot empty build/%s/shots/: %v", game, err))
		return c.summary("shot")
	}
	extras := ""
	if input != "" {
		extras += ", playing " + shortInput(input)
	}
	if savePath != "" {
		extras += ", starting with the data in " + c.shown(savePath)
	}
	if scale > 1 {
		extras += fmt.Sprintf(", enlarged %d times", scale)
	}
	c.check("info", fmt.Sprintf("running %s for %d frame(s) in a hidden window%s", game, frames[len(frames)-1], extras))
	list := make([]string, len(frames))
	for i, frame := range frames {
		list[i] = strconv.Itoa(frame)
	}
	// golib.Run reads these variables and takes the screenshots.
	env := c.gameEnv(game, "GOLIB_SHOT_DIR="+shots, "GOLIB_SHOT_FRAMES="+strings.Join(list, ","))
	if input != "" {
		env = append(env, "GOLIB_SHOT_INPUT="+input)
	}
	if savePath != "" {
		env = append(env, "GOLIB_SHOT_SAVE="+savePath)
	}
	if scale > 1 {
		env = append(env, "GOLIB_SHOT_SCALE="+strconv.Itoa(scale))
	}
	// A game that never finishes is stopped, so whoever waits for shot isn't
	// stuck.
	code, timedOut, err := c.runGame(gameRun{exe: exe, dir: c.path("games", game), env: env, timeout: shotTimeout})
	switch {
	case err != nil:
		c.check("fail", fmt.Sprintf("cannot start %s: %v", c.shown(exe), err))
	case timedOut:
		c.check("fail", fmt.Sprintf("%s didn't finish within %d seconds and was stopped (does Update or Draw loop forever?)", game, int(shotTimeout.Seconds())))
	case code != 0:
		c.check("fail", fmt.Sprintf("%s exited with code %d (see its output above)", game, code))
	}
	for _, frame := range frames {
		name := fmt.Sprintf("frame-%06d.png", frame)
		if isFile(filepath.Join(shots, name)) {
			c.check("ok", fmt.Sprintf("frame %d: build/%s/shots/%s", frame, game, name))
		} else {
			c.check("fail", fmt.Sprintf("frame %d: no screenshot was saved", frame))
		}
	}
	return c.summary("shot")
}

// shotOptionUsage explains an option of shot that was given no value.
func shotOptionUsage(option string) string {
	switch option {
	case "--save":
		return `shot --save needs a JSON file with the data the game starts with, for example: --save shots/level8.json`
	case "--scale":
		return fmt.Sprintf("shot --scale needs a whole number from 1 to %d, which the screenshots are enlarged by, for example: --scale 3", shotMaxScale)
	default:
		return `shot --input needs input to play, for example: --input "Enter@1 Right@30-90"`
	}
}

// shortInputLength is how much of an --input script shot repeats in its
// output. Scripts that play a whole level can be thousands of characters.
const shortInputLength = 100

// shortInput returns input as shot's output shows it: whole when it is short,
// otherwise its first events and how many there are.
func shortInput(input string) string {
	events := strings.Fields(input)
	if len(input) <= shortInputLength {
		return strings.Join(events, " ")
	}
	shown := ""
	for _, event := range events {
		if len(shown)+len(event) > shortInputLength {
			break
		}
		shown += event + " "
	}
	return fmt.Sprintf("%s... (%d events)", shown, len(events))
}
