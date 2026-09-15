//go:build !golib_dist

package golib

// distBuild is false in debug builds: golib build, run, shot and test.
const distBuild = false
