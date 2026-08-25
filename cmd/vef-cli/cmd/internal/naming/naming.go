package naming

import (
	"strings"
	"unicode"
)

// initialisms are the acronyms Go style keeps fully capitalized when they form
// a whole word of an identifier. Deriving a field name from a column name has
// to agree with them or the generated model reads as hand-written code that a
// reviewer would immediately correct (Id, Api, Url).
var initialisms = map[string]string{
	"acl": "ACL", "api": "API", "ascii": "ASCII", "cpu": "CPU", "css": "CSS",
	"db": "DB", "dns": "DNS", "eof": "EOF", "guid": "GUID", "html": "HTML",
	"http": "HTTP", "https": "HTTPS", "id": "ID", "ip": "IP", "json": "JSON",
	"jwt": "JWT", "lhs": "LHS", "qps": "QPS", "ram": "RAM", "rhs": "RHS",
	"rpc": "RPC", "sla": "SLA", "smtp": "SMTP", "sql": "SQL", "ssh": "SSH",
	"tcp": "TCP", "tls": "TLS", "ttl": "TTL", "udp": "UDP", "ui": "UI",
	"uid": "UID", "uri": "URI", "url": "URL", "utf8": "UTF8", "uuid": "UUID",
	"vm": "VM", "xml": "XML", "xsrf": "XSRF", "xss": "XSS",
}

// PascalCase converts a snake_case identifier into its Go exported form,
// capitalizing recognized acronyms.
//
// Two consecutive acronyms deliberately do not both stay capitalized: only the
// first keeps its uppercase form and the rest are Pascal-cased, so json_api
// becomes JSONApi rather than JSONAPI. That is the repository's identifier
// convention, and it also keeps most names reversible under bun's underscore
// rule, which only inserts a separator next to a lowercase letter.
//
// It is NOT reversible in general, and callers must not assume it is: api_v1
// becomes APIV1, which underscores back to apiv1, because the boundary between
// an acronym and a following word with no lowercase second rune leaves no
// trace. A caller that depends on the mapping must verify it with SnakeCase
// and state the original name explicitly when it does not hold.
func PascalCase(snake string) string {
	var (
		out           strings.Builder
		acronymBefore bool
	)

	for _, word := range splitWords(snake) {
		upper, isAcronym := initialisms[word]

		switch {
		case isAcronym && !acronymBefore:
			out.WriteString(upper)
		default:
			out.WriteString(capitalize(word))
		}

		acronymBefore = isAcronym
	}

	return exportable(out.String())
}

// exportable escapes a name that would not be a legal exported Go identifier.
//
// A column may legitimately begin with a digit (2fa_enabled, 3d_position) and
// Go identifiers may not, so such a name is prefixed rather than rejected —
// the caller states the real column explicitly in the bun tag, which it must
// do for any name that does not derive back anyway.
func exportable(name string) string {
	if name == "" {
		return name
	}

	if first := []rune(name)[0]; unicode.IsDigit(first) {
		return "Col" + name
	}

	return name
}

// CamelCase converts a snake_case identifier into its JSON field form.
//
// Acronyms are deliberately NOT kept in their Go spelling here: the wire name
// for tenant_id is tenantId and for base_url it is baseUrl, which is what the
// framework's own models carry everywhere. Deriving the JSON name by
// lowercasing the leading run of the Go identifier would instead produce
// tenantID and baseURL — a different wire contract from the one every existing
// client is written against. The Go field name and the JSON name follow
// different conventions, so this does not go through PascalCase.
func CamelCase(snake string) string {
	var out strings.Builder

	for i, word := range splitWords(snake) {
		if i == 0 {
			out.WriteString(word)

			continue
		}

		out.WriteString(capitalize(word))
	}

	return out.String()
}

// EntityFromTable derives the entity's snake_case name from a table name,
// dropping the module's own prefix when the table carries one: md_holiday in
// module md is the holiday entity, while sys_user in module md keeps its name
// because the prefix names a different module.
func EntityFromTable(table, module string) string {
	entity := strings.ToLower(strings.TrimSpace(table))
	if module == "" {
		return entity
	}

	prefix := strings.ToLower(module) + "_"
	if trimmed, found := strings.CutPrefix(entity, prefix); found && trimmed != "" {
		return trimmed
	}

	return entity
}

// TableAlias derives a table's query alias from the initials of its words,
// which is the convention the framework's models follow (md_allowance_item is
// aliased mai). A single-word table keeps its first two letters so the alias
// stays distinguishable from a column reference.
func TableAlias(table string) string {
	words := splitWords(table)
	if len(words) == 0 {
		return ""
	}

	if len(words) == 1 {
		word := words[0]
		if len(word) < 2 {
			return word
		}

		return word[:2]
	}

	var alias strings.Builder
	for _, word := range words {
		alias.WriteByte(word[0])
	}

	return alias.String()
}

// FileName returns the source file a snake_case entity belongs in.
func FileName(entity string) string {
	return entity + ".go"
}

// SnakeCase converts a Go identifier into the snake_case form bun derives
// column and table names with. It mirrors bun's internal.Underscore: a
// separator goes in only before an uppercase letter that neighbors a lowercase
// one, so digits are never split off the word they belong to (Sha256Sum stays
// sha256_sum).
func SnakeCase(name string) string {
	out := make([]byte, 0, len(name)+5)

	for i := range len(name) {
		c := name[i]

		switch {
		case !isASCIIUpper(c):
			out = append(out, c)
		case i > 0 && i+1 < len(name) && (isASCIILower(name[i-1]) || isASCIILower(name[i+1])):
			out = append(out, '_', c+('a'-'A'))
		default:
			out = append(out, c+('a'-'A'))
		}
	}

	return string(out)
}

// splitWords breaks a snake_case, kebab-case or space-separated identifier into
// its lowercase words, dropping empty segments so a doubled separator or a
// trailing one cannot produce one.
func splitWords(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})

	return fields
}

func capitalize(word string) string {
	if word == "" {
		return ""
	}

	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}

func isASCIIUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func isASCIILower(c byte) bool { return c >= 'a' && c <= 'z' }
