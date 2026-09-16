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
// time, and how long a game may run before it is stopped.
const (
	shotDefaultFrame = 60
	shotTimeout      = 120 * time.Second
)

// shot runs a debug build of a game in a hidden window and saves screenshots
// of chosen frames (see docs/tooling.md#screenshots). Its options are frame
// numbers, --input and a game name, in any order.
func (c *cli) shot(options []string) int {
	var names []string
	var frames []int
	input := ""
	for i := 0; i < len(options); i++ {
		option := options[i]
		switch {
		case option == "":
			return c.usage("shot got an empty argument")
		case option == "--input":
			if i+1 >= len(options) {
				return c.usage(`shot --input needs input to play, for example: --input "Enter@1 Right@30-90"`)
			}
			i++
			input = options[i]
		case strings.HasPrefix(option, "--input="):
			input = strings.TrimPrefix(option, "--input=")
		case strings.HasPrefix(option, "-"):
			return c.usage("unknown option for shot: " + option)
		case strings.Trim(option, "0123456789") == "":
			frame, _ := strconv.Atoi(option)
			if len(option) > 6 || frame < 1 {
				return c.usage(fmt.Sprintf("frame numbers go from 1 to 999999 (got: %s)", option))
			}
			frames = append(frames, frame)
		default:
			names = append(names, option)
		}
	}
	if len(frames) == 0 {
		frames = []int{shotDefaultFrame}
	}
	slices.Sort(frames)
	frames = slices.Compact(frames)
	game, exitCode := c.resolveGame("shot", names)
	if game == "" {
		return exitCode
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
	playing := ""
	if input != "" {
		playing = ", playing " + input
	}
	c.check("info", fmt.Sprintf("running %s for %d frame(s) in a hidden window%s", game, frames[len(frames)-1], playing))
	list := make([]string, len(frames))
	for i, frame := range frames {
		list[i] = strconv.Itoa(frame)
	}
	// golib.Run reads these variables and takes the screenshots.
	env := c.gameEnv(game, "GOLIB_SHOT_DIR="+shots, "GOLIB_SHOT_FRAMES="+strings.Join(list, ","))
	if input != "" {
		env = append(env, "GOLIB_SHOT_INPUT="+input)
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
