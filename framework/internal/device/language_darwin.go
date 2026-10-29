//go:build !js

package device

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// The Objective-C messages SystemLanguages sends, registered once.
var (
	selPreferredLanguages = objc.RegisterName("preferredLanguages")
	selCount              = objc.RegisterName("count")
	selObjectAtIndex      = objc.RegisterName("objectAtIndex:")
	selUTF8String         = objc.RegisterName("UTF8String")
	selInit               = objc.RegisterName("init")
	selDrain              = objc.RegisterName("drain")
)

// SystemLanguages returns the languages macOS shows its own text in, most
// preferred first, as it names them, such as "es-ES": the preferred
// languages of System Settings, from NSLocale. It needs no window, so a game
// can ask before golib.Run.
func SystemLanguages() []string {
	// What NSLocale returns is autoreleased: a pool of its own frees it, since
	// the game may ask before AppKit has one. A pool must be drained on the
	// thread that made it, and Go may move a goroutine to another thread
	// between two calls, so the goroutine stays on its thread until then.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(selAlloc).Send(selInit)
	defer pool.Send(selDrain)
	list := objc.ID(objc.GetClass("NSLocale")).Send(selPreferredLanguages)
	if list == 0 {
		return nil
	}
	count := objc.Send[uint](list, selCount)
	languages := make([]string, 0, count)
	for i := range count {
		if name := cString(objc.Send[*byte](list.Send(selObjectAtIndex, i), selUTF8String)); name != "" {
			languages = append(languages, name)
		}
	}
	return languages
}

// cString returns the text of a C string, which ends at a zero byte.
func cString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
