package app

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
)

// Android binary XML (AXML) chunk types we care about.
const (
	axmlChunkStringPool  = 0x0001
	axmlChunkStartElem   = 0x0102
	axmlStringPoolUTF8   = 0x0100     // flags bit: string data is UTF-8
	axmlNoEntry          = 0xffffffff // ResStringPool_ref "no value"
	axmlResValueString   = 0x03       // Res_value dataType TYPE_STRING
	axmlAttributeSize    = 20         // sizeof(ResXMLTree_attribute)
	axmlStartElemMinSize = 36         // header(8)+line(4)+comment(4)+ns(4)+name(4)+attrExt fields
)

// apkPackageName extracts the application package name from an APK's binary
// AndroidManifest.xml. It is self-contained (no aapt/apkanalyzer dependency)
// so it works on any machine that can build the app.
func apkPackageName(apkPath string) (string, error) {
	zr, err := zip.OpenReader(apkPath)
	if err != nil {
		return "", fmt.Errorf("打开 APK 失败：%w", err)
	}
	defer zr.Close()

	var manifest *zip.File
	for _, f := range zr.File {
		if f.Name == "AndroidManifest.xml" {
			manifest = f
			break
		}
	}
	if manifest == nil {
		return "", errors.New("APK 内未找到 AndroidManifest.xml")
	}

	rc, err := manifest.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return parseManifestPackage(data)
}

// parseManifestPackage walks the AXML chunks: it reads the string pool, then
// finds the <manifest> start element and returns its package attribute.
func parseManifestPackage(data []byte) (string, error) {
	if len(data) < 8 {
		return "", errors.New("AndroidManifest.xml 太短")
	}
	le := binary.LittleEndian
	var strs []string
	// The outer file header is 8 bytes; chunks follow.
	for pos := 8; pos+8 <= len(data); {
		ctype := le.Uint16(data[pos:])
		csize := int(le.Uint32(data[pos+4:]))
		if csize < 8 || pos+csize > len(data) {
			break
		}
		chunk := data[pos : pos+csize]
		switch ctype {
		case axmlChunkStringPool:
			s, err := parseStringPool(chunk)
			if err != nil {
				return "", err
			}
			strs = s
		case axmlChunkStartElem:
			pkg, ok, err := manifestPackageAttr(chunk, strs)
			if err != nil {
				return "", err
			}
			if ok {
				return pkg, nil
			}
		}
		pos += csize
	}
	return "", errors.New("未能从 AndroidManifest.xml 解析出包名")
}

func parseStringPool(chunk []byte) ([]string, error) {
	le := binary.LittleEndian
	if len(chunk) < 28 {
		return nil, errors.New("字符串池头部损坏")
	}
	stringCount := int(le.Uint32(chunk[8:]))
	flags := le.Uint32(chunk[16:])
	stringsStart := int(le.Uint32(chunk[20:]))
	isUTF8 := flags&axmlStringPoolUTF8 != 0

	const offsetsBase = 28
	strs := make([]string, 0, stringCount)
	for i := 0; i < stringCount; i++ {
		op := offsetsBase + i*4
		if op+4 > len(chunk) {
			return nil, errors.New("字符串偏移越界")
		}
		start := stringsStart + int(le.Uint32(chunk[op:]))
		if start < 0 || start >= len(chunk) {
			return nil, errors.New("字符串数据越界")
		}
		if isUTF8 {
			strs = append(strs, decodeUTF8String(chunk, start))
		} else {
			strs = append(strs, decodeUTF16String(chunk, start))
		}
	}
	return strs, nil
}

// decodeLen8 reads a UTF-8 pool length prefix (1 or 2 bytes) and returns the
// value plus how many bytes it consumed.
func decodeLen8(chunk []byte, p int) (int, int) {
	if p >= len(chunk) {
		return 0, 0
	}
	b := int(chunk[p])
	if b&0x80 != 0 {
		if p+1 >= len(chunk) {
			return b & 0x7f, 1
		}
		return ((b & 0x7f) << 8) | int(chunk[p+1]), 2
	}
	return b, 1
}

func decodeUTF8String(chunk []byte, p int) string {
	// First length = character count (unused), second length = byte count.
	_, n := decodeLen8(chunk, p)
	p += n
	byteLen, n := decodeLen8(chunk, p)
	p += n
	if byteLen < 0 || p+byteLen > len(chunk) {
		byteLen = len(chunk) - p
	}
	if byteLen <= 0 {
		return ""
	}
	return string(chunk[p : p+byteLen])
}

func decodeUTF16String(chunk []byte, p int) string {
	le := binary.LittleEndian
	if p+2 > len(chunk) {
		return ""
	}
	count := int(le.Uint16(chunk[p:]))
	p += 2
	if count&0x8000 != 0 {
		if p+2 > len(chunk) {
			return ""
		}
		count = ((count & 0x7fff) << 16) | int(le.Uint16(chunk[p:]))
		p += 2
	}
	units := make([]uint16, 0, count)
	for i := 0; i < count && p+2 <= len(chunk); i++ {
		units = append(units, le.Uint16(chunk[p:]))
		p += 2
	}
	return string(utf16.Decode(units))
}

// manifestPackageAttr returns the package attribute of a <manifest> start
// element. ok is false (without error) when the chunk is not the manifest
// element, so the caller keeps scanning.
func manifestPackageAttr(chunk []byte, strs []string) (string, bool, error) {
	le := binary.LittleEndian
	if len(chunk) < axmlStartElemMinSize {
		return "", false, nil
	}
	nameRef := le.Uint32(chunk[20:])
	if int(nameRef) >= len(strs) || strs[nameRef] != "manifest" {
		return "", false, nil
	}
	attrStart := int(le.Uint16(chunk[24:]))
	attrSize := int(le.Uint16(chunk[26:]))
	attrCount := int(le.Uint16(chunk[28:]))
	if attrSize == 0 {
		attrSize = axmlAttributeSize
	}
	base := 16 + attrStart // attributeStart is relative to the attrExt struct
	for i := 0; i < attrCount; i++ {
		ap := base + i*attrSize
		if ap+axmlAttributeSize > len(chunk) {
			break
		}
		ns := le.Uint32(chunk[ap:])
		name := le.Uint32(chunk[ap+4:])
		// The manifest package attribute lives in no namespace.
		if ns != axmlNoEntry || int(name) >= len(strs) || strs[name] != "package" {
			continue
		}
		if raw := le.Uint32(chunk[ap+8:]); raw != axmlNoEntry && int(raw) < len(strs) {
			return strs[raw], true, nil
		}
		if chunk[ap+15] == axmlResValueString {
			if data := le.Uint32(chunk[ap+16:]); int(data) < len(strs) {
				return strs[data], true, nil
			}
		}
		return "", false, errors.New("package 属性值无法解析")
	}
	return "", false, errors.New("AndroidManifest.xml 中未找到 package 属性")
}
