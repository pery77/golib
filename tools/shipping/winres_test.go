package main

import (
	"bytes"
	"debug/pe"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

// readResources walks a resource section as Windows does and returns every
// resource in it, in directory order. Data addresses are read as offsets in
// the section, which is what they hold before the linker relocates them.
func readResources(t *testing.T, section []byte) []resource {
	t.Helper()
	type entry struct {
		id    uint32
		name  string
		child uint32
	}
	readDirectory := func(offset uint32) []entry {
		named, numbered := int(le.Uint16(section[offset+12:])), int(le.Uint16(section[offset+14:]))
		var entries []entry
		for i := range named + numbered {
			at := offset + 16 + 8*uint32(i)
			nameOrID, child := le.Uint32(section[at:]), le.Uint32(section[at+4:])
			e := entry{child: child}
			if nameOrID&0x80000000 != 0 {
				if i >= named {
					t.Errorf("entry %d of the directory at %d has a name, but comes after the numbered ones", i, offset)
				}
				at := nameOrID &^ 0x80000000
				units := make([]uint16, le.Uint16(section[at:]))
				for c := range units {
					units[c] = le.Uint16(section[at+2+2*uint32(c):])
				}
				e.name = string(utf16.Decode(units))
			} else {
				e.id = nameOrID
			}
			entries = append(entries, e)
		}
		return entries
	}

	var resources []resource
	for _, kind := range readDirectory(0) {
		if kind.child&0x80000000 == 0 {
			t.Fatalf("type %d points at data, not at a directory", kind.id)
		}
		for _, item := range readDirectory(kind.child &^ 0x80000000) {
			languages := readDirectory(item.child &^ 0x80000000)
			if len(languages) != 1 || languages[0].id != resourceLanguage {
				t.Errorf("resource %d/%d%s has languages %+v, want only US English", kind.id, item.id, item.name, languages)
				continue
			}
			dataEntry := languages[0].child
			address, size := le.Uint32(section[dataEntry:]), le.Uint32(section[dataEntry+4:])
			if address%8 != 0 {
				t.Errorf("resource %d/%d%s starts at %d, which isn't aligned to 8 bytes", kind.id, item.id, item.name, address)
			}
			resources = append(resources, resource{
				kind: uint16(kind.id),
				id:   uint16(item.id),
				name: item.name,
				data: section[address : address+size],
			})
		}
	}
	return resources
}

func TestResourceSection(t *testing.T) {
	resources := []resource{
		{kind: rtVersion, id: 1, data: []byte("version")},
		{kind: rtIcon, id: 2, data: []byte("second icon")},
		{kind: rtIcon, id: 1, data: []byte("first")},
		{kind: rtGroupIcon, id: 7, data: []byte("numbered group")},
		{kind: rtGroupIcon, name: "Zebra", data: []byte("z")},
		{kind: rtGroupIcon, name: iconGroupName, data: []byte("icons")},
	}
	section, addressFields := resourceSection(resources)
	if len(section)%8 != 0 {
		t.Errorf("section is %d bytes, want a multiple of 8", len(section))
	}

	got := readResources(t, section)
	want := []resource{
		{kind: rtIcon, id: 1, data: []byte("first")},
		{kind: rtIcon, id: 2, data: []byte("second icon")},
		{kind: rtGroupIcon, name: iconGroupName, data: []byte("icons")},
		{kind: rtGroupIcon, name: "ZEBRA", data: []byte("z")}, // Windows stores names in capitals
		{kind: rtGroupIcon, id: 7, data: []byte("numbered group")},
		{kind: rtVersion, id: 1, data: []byte("version")},
	}
	same := slices.EqualFunc(got, want, func(a, b resource) bool {
		return a.kind == b.kind && a.id == b.id && a.name == b.name && bytes.Equal(a.data, b.data)
	})
	if !same {
		t.Errorf("resources read back:\n%v\nwant:\n%v", got, want)
	}

	if len(addressFields) != len(resources) {
		t.Fatalf("%d address fields for %d resources", len(addressFields), len(resources))
	}
	for _, field := range addressFields {
		if address := le.Uint32(section[field:]); address == 0 || int(address) >= len(section) {
			t.Errorf("the address field at %d holds %d, which is not in the section", field, address)
		}
	}
}

func TestObjectFile(t *testing.T) {
	section, addressFields := resourceSection([]resource{
		{kind: rtVersion, id: 1, data: []byte("version")},
		{kind: rtIcon, id: 1, data: []byte("icon")},
	})
	for arch, target := range targets {
		t.Run(arch, func(t *testing.T) {
			file, err := pe.NewFile(bytes.NewReader(objectFile(target, section, addressFields)))
			if err != nil {
				t.Fatal(err)
			}
			if file.Machine != target.machine {
				t.Errorf("machine = %#x, want %#x", file.Machine, target.machine)
			}
			if len(file.Sections) != 1 || file.Sections[0].Name != ".rsrc" {
				t.Fatalf("sections = %v, want just .rsrc", file.Sections)
			}
			rsrc := file.Sections[0]
			data, err := rsrc.Data()
			if err != nil || !bytes.Equal(data, section) {
				t.Errorf("section data differs from the resource section (error %v)", err)
			}
			if rsrc.Characteristics != pe.IMAGE_SCN_CNT_INITIALIZED_DATA|pe.IMAGE_SCN_MEM_READ {
				t.Errorf("section characteristics = %#x, want initialized, read-only data", rsrc.Characteristics)
			}
			var relocated []uint32
			for _, reloc := range rsrc.Relocs {
				if reloc.SymbolTableIndex != 0 || reloc.Type != target.addressRelocation {
					t.Errorf("relocation %+v, want symbol 0 and type %#x", reloc, target.addressRelocation)
				}
				relocated = append(relocated, reloc.VirtualAddress)
			}
			if !slices.Equal(relocated, addressFields) {
				t.Errorf("relocations at %v, want %v", relocated, addressFields)
			}
			if len(file.COFFSymbols) != 1 {
				t.Fatalf("%d symbols, want the section symbol only", len(file.COFFSymbols))
			}
			symbol := file.COFFSymbols[0]
			if name, _ := symbol.FullName(file.StringTable); name != ".rsrc" || symbol.SectionNumber != 1 || symbol.StorageClass != 3 {
				t.Errorf("symbol = %q %+v, want the static symbol of section 1, .rsrc", name, symbol)
			}
		})
	}
}

// versionNode is a block of version information, read back.
type versionNode struct {
	key         string
	text        bool
	valueLength int
	value       []byte
	children    []versionNode
}

// readVersionBlock reads the block at the start of data, checking the
// lengths and the 4-byte alignment that Windows expects.
func readVersionBlock(t *testing.T, data []byte) versionNode {
	t.Helper()
	length := int(le.Uint16(data[0:]))
	if length > len(data) {
		t.Fatalf("a block says it is %d bytes, but only %d are left", length, len(data))
	}
	data = data[:length]
	node := versionNode{valueLength: int(le.Uint16(data[2:])), text: le.Uint16(data[4:]) == 1}
	at := 6
	var key []uint16
	for ; le.Uint16(data[at:]) != 0; at += 2 {
		key = append(key, le.Uint16(data[at:]))
	}
	node.key = string(utf16.Decode(key))
	at = align(at+2, 4)
	valueBytes := node.valueLength
	if node.text {
		valueBytes *= 2
	}
	node.value = data[at : at+valueBytes]
	at += valueBytes
	for at = align(at, 4); at < length; at = align(at, 4) {
		child := readVersionBlock(t, data[at:])
		node.children = append(node.children, child)
		at += int(le.Uint16(data[at:]))
	}
	return node
}

// texts returns the text values under StringFileInfo, by key.
func (n versionNode) texts(t *testing.T) map[string]string {
	t.Helper()
	values := map[string]string{}
	for _, info := range n.children {
		if info.key != "StringFileInfo" {
			continue
		}
		for _, table := range info.children {
			if table.key != "040904B0" {
				t.Errorf("string table %q, want 040904B0: US English, Unicode", table.key)
			}
			for _, text := range table.children {
				units := make([]uint16, len(text.value)/2)
				for i := range units {
					units[i] = le.Uint16(text.value[2*i:])
				}
				if len(units) == 0 || units[len(units)-1] != 0 {
					t.Errorf("%s isn't terminated by a zero", text.key)
					continue
				}
				values[text.key] = string(utf16.Decode(units[:len(units)-1]))
			}
		}
	}
	return values
}

func TestVersionInfo(t *testing.T) {
	info := gameInfo{
		Title:     "Rocks in Space",
		Author:    "Ada Lovelace",
		Copyright: "Copyright 2026 Ada Lovelace",
		version:   version{major: 1, minor: 20, patch: 3, label: "-beta"},
	}
	root := readVersionBlock(t, versionInfo(info, "rocks"))
	if root.key != "VS_VERSION_INFO" || root.text || root.valueLength != 52 {
		t.Fatalf("root block = %q, text %v, value %d bytes; want VS_VERSION_INFO with 52 bytes of fixed information", root.key, root.text, root.valueLength)
	}
	fixed := root.value
	for _, check := range []struct {
		name   string
		offset int
		want   uint32
	}{
		{"signature", 0, 0xFEEF04BD},
		{"file version, high", 8, 1<<16 | 20},
		{"file version, low", 12, 3 << 16},
		{"product version, high", 16, 1<<16 | 20},
		{"product version, low", 20, 3 << 16},
		{"flags", 28, 0x02},
		{"operating system", 32, 0x00040004},
		{"file type", 36, 1},
	} {
		if got := le.Uint32(fixed[check.offset:]); got != check.want {
			t.Errorf("%s = %#x, want %#x", check.name, got, check.want)
		}
	}

	got := root.texts(t)
	want := map[string]string{
		"CompanyName":      "Ada Lovelace",
		"FileDescription":  "Rocks in Space",
		"FileVersion":      "1.20.3-beta",
		"InternalName":     "rocks",
		"LegalCopyright":   "Copyright 2026 Ada Lovelace",
		"OriginalFilename": "rocks.exe",
		"ProductName":      "Rocks in Space",
		"ProductVersion":   "1.20.3-beta",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("strings = %v\nwant %v", got, want)
	}

	var translation []byte
	for _, child := range root.children {
		if child.key == "VarFileInfo" && len(child.children) == 1 && child.children[0].key == "Translation" {
			translation = child.children[0].value
		}
	}
	if !bytes.Equal(translation, []byte{0x09, 0x04, 0xB0, 0x04}) {
		t.Errorf("translation = % x, want 09 04 b0 04: US English, Unicode", translation)
	}

	// Empty fields are left out, and a release has no pre-release flag.
	plain := readVersionBlock(t, versionInfo(gameInfo{Title: "Rocks"}, "rocks"))
	if _, found := plain.texts(t)["CompanyName"]; found {
		t.Error("an empty author still gave a CompanyName")
	}
	if flags := le.Uint32(plain.value[28:]); flags != 0 {
		t.Errorf("flags = %#x for a release, want 0", flags)
	}
}

func TestVersionInfoHandlesAnyText(t *testing.T) {
	// Titles of every length land on the 4-byte boundaries, including ones
	// outside the Basic Multilingual Plane.
	for n := range 9 {
		title := strings.Repeat("é", n) + "🚀"
		got := readVersionBlock(t, versionInfo(gameInfo{Title: title}, "rocks")).texts(t)
		if got["ProductName"] != title {
			t.Errorf("ProductName = %q, want %q", got["ProductName"], title)
		}
	}
}
