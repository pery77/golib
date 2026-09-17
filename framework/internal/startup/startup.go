// Package startup makes sure a game that can't start says why.
//
// raylib-go and ffi load the raylib and libffi libraries while Go initializes
// their packages, before main and before golib.Run, and panic when they
// can't. A Windows dist build has no console, so the player would see nothing
// happen. On Windows, this package loads those libraries first, and when one
// doesn't load, it says so in a message box when nobody would see the
// console, and on the console otherwise, then ends the program.
//
// That only works because this package is initialized first. Go initializes
// packages in the order of their import paths, each as soon as the packages
// it imports are initialized. So this package, golib/internal/startup, goes
// right after syscall, before internal/..., os and unicode, and so before fmt,
// ffi and raylib-go, which need os, as long as it needs nothing initialized
// later. It imports only syscall and unsafe, and a test keeps it that way:
// even strings would hold it back, because strings needs unicode, which comes
// after os.
//
// golib shows its own errors with ShowError and ErrorsUnseen.
package startup
