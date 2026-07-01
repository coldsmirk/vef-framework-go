package modelschema

import "strings"

// bunTag mirrors bun v1.2.18's internal/tagparser.Tag. The generator must decide
// which token of a bun struct tag is the column/table name and which are
// key:value options exactly the way the ORM does, so it replicates bun's tag
// parser verbatim — that package is internal and cannot be imported, the same
// reason underscore() replicates bun's internal.Underscore. Keeping the two in
// lockstep is what guarantees generated names never diverge from the names bun
// binds at runtime; in particular the parser does NOT trim whitespace (a leading
// space makes " scanonly" an unrecognized option, not the scanonly flag) and the
// last value wins for a repeated option.
type bunTag struct {
	Name    string
	Options map[string][]string
}

// hasOption reports whether the tag carries the given option key.
func (t bunTag) hasOption(name string) bool {
	_, ok := t.Options[name]

	return ok
}

// option returns the last value bound to the given option key (bun's last-wins
// semantics for repeated options) and whether the key was present at all.
func (t bunTag) option(name string) (string, bool) {
	if vs, ok := t.Options[name]; ok {
		return vs[len(vs)-1], true
	}

	return "", false
}

// parseBunStructTag parses a raw bun tag body (the text inside `bun:"..."`) into
// its name and options, byte-for-byte as bun's tagparser.Parse does.
func parseBunStructTag(s string) bunTag {
	if s == "" {
		return bunTag{}
	}

	p := bunTagParser{s: s}
	p.parse()

	return p.tag
}

type bunTagParser struct {
	s        string
	i        int
	tag      bunTag
	seenName bool
}

func (p *bunTagParser) setName(name string) {
	if p.seenName {
		p.addOption(name, "")
	} else {
		p.seenName = true
		p.tag.Name = name
	}
}

func (p *bunTagParser) addOption(key, value string) {
	p.seenName = true

	if key == "" {
		return
	}

	if p.tag.Options == nil {
		p.tag.Options = make(map[string][]string)
	}

	if vs, ok := p.tag.Options[key]; ok {
		p.tag.Options[key] = append(vs, value)
	} else {
		p.tag.Options[key] = []string{value}
	}
}

func (p *bunTagParser) parse() {
	for p.valid() {
		p.parseKeyValue()

		if p.peek() == ',' {
			p.i++
		}
	}
}

func (p *bunTagParser) parseKeyValue() {
	start := p.i

	for p.valid() {
		switch c := p.read(); c {
		case ',':
			p.setName(p.s[start : p.i-1])

			return
		case ':':
			key := p.s[start : p.i-1]
			p.addOption(key, p.parseValue())

			return

		case '"':
			p.setName(p.parseQuotedValue())

			return
		}
	}

	p.setName(p.s[start:p.i])
}

func (p *bunTagParser) parseValue() string {
	start := p.i

	for p.valid() {
		switch c := p.read(); c {
		case '"':
			return p.parseQuotedValue()
		case ',':
			return p.s[start : p.i-1]
		case '(':
			p.skipPairs('(', ')')
		}
	}

	if p.i == start {
		return ""
	}

	return p.s[start:p.i]
}

func (p *bunTagParser) parseQuotedValue() string {
	if i := strings.IndexByte(p.s[p.i:], '"'); i >= 0 && p.s[p.i+i-1] != '\\' {
		s := p.s[p.i : p.i+i]
		p.i += i + 1

		return s
	}

	b := make([]byte, 0, 16)

	for p.valid() {
		switch c := p.read(); c {
		case '\\':
			b = append(b, p.read())
		case '"':
			return string(b)
		default:
			b = append(b, c)
		}
	}

	return ""
}

func (p *bunTagParser) skipPairs(start, end byte) {
	var lvl int

	for p.valid() {
		switch c := p.read(); c {
		case '"':
			_ = p.parseQuotedValue()
		case start:
			lvl++
		case end:
			if lvl == 0 {
				return
			}

			lvl--
		}
	}
}

func (p *bunTagParser) valid() bool { return p.i < len(p.s) }

func (p *bunTagParser) read() byte {
	if !p.valid() {
		return 0
	}

	c := p.s[p.i]
	p.i++

	return c
}

func (p *bunTagParser) peek() byte {
	if !p.valid() {
		return 0
	}

	return p.s[p.i]
}
