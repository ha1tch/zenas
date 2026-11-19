package assembler

import "testing"

func TestTagDefines(t *testing.T) {
	d, err := TagDefines([]string{"debug", "plus3", "debug"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]uint16{"ZENAS_TAG_debug": 1, "ZENAS_TAGBIT_debug": 1, "ZENAS_TAG_plus3": 1, "ZENAS_TAGBIT_plus3": 2, "ZENAS_TAGS": 3}
	if len(d) != len(want) {
		t.Fatalf("%v", d)
	}
	for k, v := range want {
		if d[k] != v {
			t.Errorf("%s = %d, want %d", k, d[k], v)
		}
	}
	if d, err := TagDefines(nil); err != nil || len(d) != 1 || d["ZENAS_TAGS"] != 0 {
		t.Errorf("no tags: %v %v", d, err)
	}
	if _, err := TagDefines([]string{"9lives"}); err == nil {
		t.Error("a tag that starts with a digit is not an error")
	}
	many := make([]string, 17)
	for i := range many {
		many[i] = "t" + string(rune('a'+i))
	}
	if _, err := TagDefines(many); err == nil {
		t.Error("17 tags are not an error")
	}
}

func TestDefineTagsAssembles(t *testing.T) {
	const source = `        ORG 0x8000
        IFDEF ZENAS_TAG_extra
        DB 1
        ENDIF
        DB ZENAS_TAGS
`
	for _, c := range []struct {
		tags []string
		want []byte
	}{{nil, []byte{0}}, {[]string{"extra"}, []byte{1, 1}}} {
		asm := New()
		if err := asm.DefineTags(c.tags); err != nil {
			t.Fatal(err)
		}
		r, err := asm.AssembleString(source)
		if err != nil {
			t.Fatal(err)
		}
		if string(r.MachineCode) != string(c.want) {
			t.Errorf("tags %v: % X, want % X", c.tags, r.MachineCode, c.want)
		}
	}
}

func TestParseDefine(t *testing.T) {
	for _, c := range []struct {
		in   string
		name string
		val  uint16
	}{{"DEBUG", "DEBUG", 1}, {"N=42", "N", 42}, {"ADDR=0x8000", "ADDR", 0x8000}, {" X = $FF ", "X", 0xFF}} {
		name, val, err := ParseDefine(c.in)
		if err != nil || name != c.name || val != c.val {
			t.Errorf("%q: %q %d %v", c.in, name, val, err)
		}
	}
	for _, bad := range []string{"", "=1", "N=abc"} {
		if _, _, err := ParseDefine(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
