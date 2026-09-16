package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// The Go modules that ship the shared libraries every game loads when it
// starts: raylib, and libffi, which raylib-go calls it through.
const (
	raylibModule = "github.com/gen2brain/raylib-go/raylib"
	ffiModule    = "github.com/jupiterrider/ffi"
)

// raylibArchives name the archive in raylib-go's libs/ folder that holds the
// raylib library for each platform: libs/raylib-<version>_<name>.tar.gz.
var raylibArchives = map[string]string{
	"windows/amd64": "win64_msvc16",
	"windows/arm64": "winarm64_msvc16",
	"linux/amd64":   "linux_amd64",
	"linux/arm64":   "linux_arm64",
	"darwin/amd64":  "macos",
	"darwin/arm64":  "macos",
}

// libffiFiles name the libffi library in the ffi module for each platform
// that it ships one for. Linux games load the system's libffi.so.8.
var libffiFiles = map[string]string{
	"windows/amd64": "assets/libffi/windows_amd64/libffi-8.dll",
	"darwin/amd64":  "assets/libffi/darwin_amd64/libffi.8.dylib",
	"darwin/arm64":  "assets/libffi/darwin_arm64/libffi.8.dylib",
}

// goModule is what go list says about a Go module.
type goModule struct {
	Path    string
	Version string
	Main    bool
	Dir     string
}

// library is a shared library that a game loads when it starts.
type library struct {
	name    string // its file name, such as raylib.dll
	title   string // what it is, such as "raylib 6.0"
	url     string
	from    goModule // the Go module it comes with
	license string   // the path of its license text
	// archive is the .tar.gz file that holds it as its only file, or "" when
	// file is the library itself.
	archive string
	file    string
}

// findLibraries returns the shared libraries that games built for platform,
// such as "windows/amd64", load, found in modules: raylib, and libffi where
// the ffi module ships it.
func findLibraries(platform string, modules []goModule) ([]library, error) {
	var libraries []library
	for _, m := range modules {
		switch m.Path {
		case raylibModule:
			suffix, found := raylibArchives[platform]
			if !found {
				return nil, fmt.Errorf("%s has no raylib library for %s", m.Path, platform)
			}
			archives, _ := filepath.Glob(filepath.Join(m.Dir, "libs", "raylib-*_"+suffix+".tar.gz"))
			if len(archives) != 1 {
				return nil, fmt.Errorf("%s %s has no single libs/raylib-*_%s.tar.gz (run: golib setup)", m.Path, m.Version, suffix)
			}
			name, err := archivedFileName(archives[0])
			if err != nil {
				return nil, err
			}
			version := strings.TrimPrefix(filepath.Base(archives[0]), "raylib-")
			version, _, _ = strings.Cut(version, "_")
			libraries = append(libraries, library{
				name:    name,
				title:   "raylib " + version,
				url:     "https://www.raylib.com",
				from:    m,
				license: filepath.Join(m.Dir, "libs", "LICENSE"),
				archive: archives[0],
			})
		case ffiModule:
			file, found := libffiFiles[platform]
			if !found {
				continue
			}
			libraries = append(libraries, library{
				name:    filepath.Base(file),
				title:   "libffi",
				url:     "https://sourceware.org/libffi/",
				from:    m,
				license: filepath.Join(m.Dir, "assets", "libffi", "LICENSE"),
				file:    filepath.Join(m.Dir, filepath.FromSlash(file)),
			})
		}
	}
	return libraries, nil
}

// archivedFileName returns the name of the first file in a .tar.gz archive.
func archivedFileName(archive string) (string, error) {
	var name string
	err := readArchive(archive, func(header *tar.Header, _ io.Reader) error {
		name = header.Name
		return nil
	})
	return name, err
}

// readArchive calls read with the first file in a .tar.gz archive.
func readArchive(archive string, read func(header *tar.Header, content io.Reader) error) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("%s: %w", archive, err)
	}
	defer unzipped.Close()
	files := tar.NewReader(unzipped)
	header, err := files.Next()
	if err != nil {
		return fmt.Errorf("%s: %w", archive, err)
	}
	if header.Typeflag != tar.TypeReg || header.Name != filepath.Base(header.Name) {
		return fmt.Errorf("%s should hold a single library, but starts with %q", archive, header.Name)
	}
	return read(header, files)
}

// copyTo writes the library into dir, under its own name.
func (l library) copyTo(dir string) error {
	target := filepath.Join(dir, l.name)
	if l.archive == "" {
		source, err := os.Open(l.file)
		if err != nil {
			return err
		}
		defer source.Close()
		return createFile(target, source, 0o644)
	}
	return readArchive(l.archive, func(header *tar.Header, content io.Reader) error {
		return createFile(target, content, header.FileInfo().Mode().Perm()|0o644)
	})
}

// createFile writes content to a new file at path.
func createFile(path string, content io.Reader, perm os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, content)
	return errors.Join(err, file.Close())
}
