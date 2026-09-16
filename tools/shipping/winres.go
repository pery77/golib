package main

import (
	"bytes"
	"cmp"
	"debug/pe"
	"encoding/binary"
	"slices"
	"strings"
	"unicode/utf16"
)

// Windows resource types.
const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
)

// resourceLanguage is US English, the language every resource is filed under.
const resourceLanguage = 0x0409

// resource is one of a program's resources, found by its type and by either a
// number or a name.
type resource struct {
	kind uint16
	id   uint16 // when name is ""
	name string
	data []byte
}

// target is what an object file says about the processor it is for.
type target struct {
	machine uint16
	// addressRelocation turns an offset in a section into an address relative
	// to the start of the program, which is how resources point to their data.
	addressRelocation uint16
}

// targets are the processors GoLib builds Windows games for, by Go name.
var targets = map[string]target{
	"amd64": {machine: pe.IMAGE_FILE_MACHINE_AMD64, addressRelocation: 0x0003}, // IMAGE_REL_AMD64_ADDR32NB
	"arm64": {machine: pe.IMAGE_FILE_MACHINE_ARM64, addressRelocation: 0x0002}, // IMAGE_REL_ARM64_ADDR32NB
}

var le = binary.LittleEndian

// resourceSection lays resources out as the .rsrc section of a Windows
// program: a directory of types, then one of names or numbers within each
// type, then one of languages, then the data. It returns the section and
// where, in it, each resource's data address goes; the linker fills those in.
func resourceSection(resources []resource) (section []byte, addressFields []uint32) {
	// Windows looks entries up by binary search: named entries come first,
	// sorted by name, then numbered ones, sorted by number.
	sorted := slices.Clone(resources)
	slices.SortStableFunc(sorted, func(a, b resource) int {
		if c := cmp.Compare(a.kind, b.kind); c != 0 {
			return c
		}
		if (a.name == "") != (b.name == "") {
			if a.name != "" {
				return -1
			}
			return 1
		}
		if c := strings.Compare(strings.ToUpper(a.name), strings.ToUpper(b.name)); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})
	var kinds []uint16
	for _, r := range sorted {
		if len(kinds) == 0 || kinds[len(kinds)-1] != r.kind {
			kinds = append(kinds, r.kind)
		}
	}

	// Work out where everything goes: directories, data entries, names, data.
	directorySize := func(entries int) int { return 16 + 8*entries }
	offset := directorySize(len(kinds))
	kindDirectories := make([]int, len(kinds))
	for i, kind := range kinds {
		kindDirectories[i] = offset
		offset += directorySize(countKind(sorted, kind))
	}
	languageDirectories := make([]int, len(sorted))
	for i := range sorted {
		languageDirectories[i] = offset
		offset += directorySize(1)
	}
	dataEntries := make([]int, len(sorted))
	for i := range sorted {
		dataEntries[i] = offset
		offset += 16
	}
	names := make([]int, len(sorted))
	for i, r := range sorted {
		if r.name != "" {
			names[i] = offset
			offset += 2 + 2*len(utf16.Encode([]rune(strings.ToUpper(r.name))))
		}
	}
	data := make([]int, len(sorted))
	for i, r := range sorted {
		offset = align(offset, 8)
		data[i] = offset
		offset += len(r.data)
	}
	section = make([]byte, align(offset, 8))

	// Write it.
	const subdirectory, namedEntry = 0x80000000, 0x80000000
	writeDirectory(section, 0, 0, len(kinds))
	next := 0 // index in sorted of the first resource of the current kind
	for i, kind := range kinds {
		entry := 16 + 8*i
		le.PutUint32(section[entry:], uint32(kind))
		le.PutUint32(section[entry+4:], subdirectory|uint32(kindDirectories[i]))

		count := countKind(sorted, kind)
		named := 0
		for _, r := range sorted[next : next+count] {
			if r.name != "" {
				named++
			}
		}
		writeDirectory(section, kindDirectories[i], named, count-named)
		for j := range count {
			r, k := sorted[next+j], next+j
			entry := kindDirectories[i] + 16 + 8*j
			if r.name != "" {
				le.PutUint32(section[entry:], namedEntry|uint32(names[k]))
				name := utf16.Encode([]rune(strings.ToUpper(r.name)))
				le.PutUint16(section[names[k]:], uint16(len(name)))
				for c, unit := range name {
					le.PutUint16(section[names[k]+2+2*c:], unit)
				}
			} else {
				le.PutUint32(section[entry:], uint32(r.id))
			}
			le.PutUint32(section[entry+4:], subdirectory|uint32(languageDirectories[k]))

			writeDirectory(section, languageDirectories[k], 0, 1)
			le.PutUint32(section[languageDirectories[k]+16:], resourceLanguage)
			le.PutUint32(section[languageDirectories[k]+20:], uint32(dataEntries[k]))

			// The data entry: address (an offset until the linker relocates
			// it), size, code page and a reserved field.
			le.PutUint32(section[dataEntries[k]:], uint32(data[k]))
			le.PutUint32(section[dataEntries[k]+4:], uint32(len(r.data)))
			copy(section[data[k]:], r.data)
			addressFields = append(addressFields, uint32(dataEntries[k]))
		}
		next += count
	}
	return section, addressFields
}

// writeDirectory writes the header of a resource directory at offset: its
// entries follow it.
func writeDirectory(section []byte, offset, named, numbered int) {
	le.PutUint16(section[offset+12:], uint16(named))
	le.PutUint16(section[offset+14:], uint16(numbered))
}

func countKind(resources []resource, kind uint16) int {
	count := 0
	for _, r := range resources {
		if r.kind == kind {
			count++
		}
	}
	return count
}

func align(n, to int) int {
	return (n + to - 1) / to * to
}

// objectFile wraps a resource section in a COFF object file, the format of the
// .syso files the Go linker reads. Each address field gets a relocation, which
// makes the linker add the section's address to the offset stored there.
func objectFile(t target, section []byte, addressFields []uint32) []byte {
	const fileHeaderSize, sectionHeaderSize, relocationSize = 20, 40, 10
	name := [8]uint8{'.', 'r', 's', 'r', 'c'}
	dataStart := fileHeaderSize + sectionHeaderSize
	relocationStart := dataStart + len(section)
	symbolStart := relocationStart + relocationSize*len(addressFields)

	var file bytes.Buffer
	write := func(value any) {
		if err := binary.Write(&file, le, value); err != nil {
			panic(err) // writing fixed-size values to memory can't fail
		}
	}
	write(pe.FileHeader{
		Machine:              t.machine,
		NumberOfSections:     1,
		PointerToSymbolTable: uint32(symbolStart),
		NumberOfSymbols:      1,
	})
	write(pe.SectionHeader32{
		Name:                 name,
		SizeOfRawData:        uint32(len(section)),
		PointerToRawData:     uint32(dataStart),
		PointerToRelocations: uint32(relocationStart),
		NumberOfRelocations:  uint16(len(addressFields)),
		Characteristics:      pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ,
	})
	file.Write(section)
	for _, field := range addressFields {
		// Symbol 0 is the section itself.
		write(pe.Reloc{VirtualAddress: field, SymbolTableIndex: 0, Type: t.addressRelocation})
	}
	write(pe.COFFSymbol{
		Name:          name,
		SectionNumber: 1,
		StorageClass:  3, // IMAGE_SYM_CLASS_STATIC: a section symbol
	})
	write(uint32(4)) // an empty string table: just its own size
	return file.Bytes()
}

// versionInfo encodes the version information of the game called name, as
// Explorer shows it in the file's properties. Task Manager and the Start menu
// show its FileDescription as the program's name, so that is the title.
func versionInfo(info gameInfo, name string) []byte {
	v := info.version
	fixed := make([]byte, 52) // VS_FIXEDFILEINFO
	le.PutUint32(fixed[0:], 0xFEEF04BD)
	le.PutUint32(fixed[4:], 0x00010000) // structure version 1.0
	high := uint32(v.major)<<16 | uint32(v.minor)
	low := uint32(v.patch) << 16
	le.PutUint32(fixed[8:], high) // file version
	le.PutUint32(fixed[12:], low)
	le.PutUint32(fixed[16:], high) // product version
	le.PutUint32(fixed[20:], low)
	le.PutUint32(fixed[24:], 0x3F) // which flags are valid
	if v.label != "" {
		le.PutUint32(fixed[28:], 0x02) // VS_FF_PRERELEASE
	}
	le.PutUint32(fixed[32:], 0x00040004) // VOS_NT_WINDOWS32
	le.PutUint32(fixed[36:], 1)          // VFT_APP: an application

	var texts [][]byte
	for _, text := range []struct{ key, value string }{
		{"CompanyName", info.Author},
		{"FileDescription", info.Title},
		{"FileVersion", v.String()},
		{"InternalName", name},
		{"LegalCopyright", info.Copyright},
		{"OriginalFilename", name + ".exe"},
		{"ProductName", info.Title},
		{"ProductVersion", v.String()},
	} {
		if text.value != "" {
			value := utf16z(text.value)
			texts = append(texts, versionBlock(text.key, 1, value, uint16(len(value)/2), nil))
		}
	}
	// The strings are US English, in Unicode (code page 1200): 0409 04B0.
	stringTable := versionBlock("040904B0", 1, nil, 0, texts)
	translation := []byte{0x09, 0x04, 0xB0, 0x04}
	return versionBlock("VS_VERSION_INFO", 0, fixed, uint16(len(fixed)), [][]byte{
		versionBlock("StringFileInfo", 1, nil, 0, [][]byte{stringTable}),
		versionBlock("VarFileInfo", 1, nil, 0, [][]byte{
			versionBlock("Translation", 0, translation, uint16(len(translation)), nil),
		}),
	})
}

// versionBlock encodes one block of version information: its length, the
// length of its value, whether the value is text (1) or binary (0), its key,
// its value and the blocks inside it, each starting on a 4-byte boundary.
// valueLength counts bytes for a binary value and characters for text.
func versionBlock(key string, text uint16, value []byte, valueLength uint16, children [][]byte) []byte {
	block := make([]byte, 6, 64)
	block = append(block, utf16z(key)...)
	block = append(block, make([]byte, align(len(block), 4)-len(block))...)
	block = append(block, value...)
	for _, child := range children {
		block = append(block, make([]byte, align(len(block), 4)-len(block))...)
		block = append(block, child...)
	}
	le.PutUint16(block[0:], uint16(len(block)))
	le.PutUint16(block[2:], valueLength)
	le.PutUint16(block[4:], text)
	return block
}

// utf16z encodes s as UTF-16, with a terminating zero, as Windows stores text.
func utf16z(s string) []byte {
	var encoded []byte
	for _, unit := range utf16.Encode([]rune(s)) {
		encoded = le.AppendUint16(encoded, unit)
	}
	return le.AppendUint16(encoded, 0)
}
