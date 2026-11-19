package assembler

import (
	"fmt"
	"strconv"
	"strings"
)

// Build tags and command-line symbols: what `zenas assemble --tag` and
// `--define` do, for a program that assembles through this package.
//
// A tag named NAME defines ZENAS_TAG_NAME as 1, for IFDEF and IF with
// AND/OR/NOT, and ZENAS_TAGBIT_NAME as that tag's bit: each distinct tag,
// in the order given, gets the next bit. ZENAS_TAGS is the OR of the bits
// and is always defined, 0 with no tags, so a source that reads it
// assembles in every configuration. Symbols are 16 bits, so there are at
// most 16 distinct tags.

// MaxTags is the number of distinct tags one assembly can have.
const MaxTags = 16

// TagSymbol returns the symbol a tag defines, ZENAS_TAG_<name>. The name
// has to be an identifier (letters, digits, underscore; no digit first),
// so that the symbol can be used in IF and IFDEF. Its case is kept.
func TagSymbol(tag string) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("a tag needs a name")
	}
	for i, r := range tag {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if r == '_' || isLetter || (isDigit && i > 0) {
			continue
		}
		return "", fmt.Errorf("invalid tag name %q: tags must be identifiers (letters, digits, underscore; not starting with a digit)", tag)
	}
	return "ZENAS_TAG_" + tag, nil
}

// TagDefines returns the symbols a list of tags defines, ZENAS_TAGS
// among them. A tag given twice counts once.
func TagDefines(tags []string) (map[string]uint16, error) {
	defines := map[string]uint16{}
	var mask uint16
	bit := 0
	for _, tag := range tags {
		sym, err := TagSymbol(tag)
		if err != nil {
			return nil, err
		}
		if _, seen := defines[sym]; seen {
			continue
		}
		if bit >= MaxTags {
			return nil, fmt.Errorf("too many distinct tags (at most %d have a bit in ZENAS_TAGS)", MaxTags)
		}
		tag = strings.TrimSpace(tag)
		defines[sym] = 1
		defines["ZENAS_TAGBIT_"+tag] = uint16(1) << uint(bit)
		mask |= uint16(1) << uint(bit)
		bit++
	}
	defines["ZENAS_TAGS"] = mask
	return defines, nil
}

// DefineTags defines the symbols of a list of build tags (TagDefines)
// for the next assembly. Call it once, with all the tags, an empty list
// included: that is what defines ZENAS_TAGS as 0.
func (a *Assembler) DefineTags(tags []string) error {
	defines, err := TagDefines(tags)
	if err != nil {
		return err
	}
	for name, value := range defines {
		a.Define(name, value)
	}
	return nil
}

// ParseDefine reads a symbol given as NAME or NAME=VALUE, the form of
// `--define`. A bare NAME is 1. VALUE is decimal, 0x hex or $ hex.
func ParseDefine(s string) (name string, value uint16, err error) {
	name = s
	valStr := "1"
	if idx := strings.IndexByte(s, '='); idx >= 0 {
		name = s[:idx]
		valStr = s[idx+1:]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", 0, fmt.Errorf("a define needs a symbol name")
	}
	valStr = strings.TrimSpace(valStr)
	var v int64
	switch {
	case strings.HasPrefix(valStr, "0x"), strings.HasPrefix(valStr, "0X"):
		v, err = strconv.ParseInt(valStr[2:], 16, 32)
	case strings.HasPrefix(valStr, "$"):
		v, err = strconv.ParseInt(valStr[1:], 16, 32)
	default:
		v, err = strconv.ParseInt(valStr, 10, 32)
	}
	if err != nil {
		return "", 0, fmt.Errorf("invalid define value %q: %v", valStr, err)
	}
	return name, uint16(v), nil
}
