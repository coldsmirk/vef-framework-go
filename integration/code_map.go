package integration

import "github.com/coldsmirk/vef-framework-go/orm"

// UnmappedPolicy declares how a code map answers a lookup no entry matches:
// reject fails the translation (fail closed, the default), passthrough returns
// the input value unchanged, fallback returns the configured fallback value of
// the lookup's target side. Scripts may override the stored policy per call.
type UnmappedPolicy string

const (
	// UnmappedPolicyReject fails an unmapped lookup with ErrUnmappedValue; an
	// empty policy resolves to this default.
	UnmappedPolicyReject UnmappedPolicy = "reject"
	// UnmappedPolicyPassthrough returns the input value unchanged.
	UnmappedPolicyPassthrough UnmappedPolicy = "passthrough"
	// UnmappedPolicyFallback returns the map's fallback value for the lookup's
	// target side.
	UnmappedPolicyFallback UnmappedPolicy = "fallback"
)

// IsValid reports whether the policy is empty (defaulting to reject) or one
// of the known policies.
func (p UnmappedPolicy) IsValid() bool {
	return p == "" || p == UnmappedPolicyReject || p == UnmappedPolicyPassthrough || p == UnmappedPolicyFallback
}

// CodeMapEntry is one bidirectional mapping pair between the canonical model
// and one external system's coding. Each side carries one primary value and
// any number of aliases: lookups match the primary or any alias, translations
// always emit the opposite side's primary — aliases are matched, never
// emitted. Values keep their JSON type (string, number, or boolean) end to
// end; lookups compare by normalized string form, so 1 and "1" address the
// same entry.
type CodeMapEntry struct {
	// Canonical is the host-side primary value, emitted by toCanonical lookups.
	Canonical any `json:"canonical"`
	// External is the external-side primary value, emitted by toExternal lookups.
	External any `json:"external"`
	// CanonicalAliases are additional host-side values matching this entry.
	CanonicalAliases []any `json:"canonicalAliases,omitempty"`
	// ExternalAliases are additional external-side values matching this entry.
	ExternalAliases []any `json:"externalAliases,omitempty"`
}

// CodeMap translates the values of one code set (gender, marital status, ...)
// between the host's canonical codes and one external system's codes — the
// code-level instance of the canonical data model pattern. Adapter scripts
// consume it through the codes library (codes.toExternal / codes.toCanonical);
// save-time validation keeps both sides collision-free so every lookup is
// deterministic.
type CodeMap struct {
	orm.BaseModel `bun:"table:itg_code_map,alias:icm"`
	orm.FullAuditedModel

	SystemID string `json:"systemId" bun:"system_id"`
	// CodeSet identifies the translated code set (e.g. "gender"). Where the
	// host catalog exposes the same set (mold translate tags, CodeSetInspector),
	// the identifiers should agree.
	CodeSet string `json:"codeSet" bun:"code_set"`
	Name    string `json:"name" bun:"name"`
	// Entries are the mapping pairs; save-time validation rejects duplicate
	// lookup values per side across primaries and aliases.
	Entries []CodeMapEntry `json:"entries" bun:"entries,type:jsonb,nullzero"`
	// OnUnmapped is the map's default behavior for lookups no entry matches;
	// empty resolves to reject.
	OnUnmapped UnmappedPolicy `json:"onUnmapped" bun:"on_unmapped"`
	// FallbackCanonical is the value toCanonical yields for unmapped input
	// under the fallback policy.
	FallbackCanonical any `json:"fallbackCanonical,omitempty" bun:"fallback_canonical,type:jsonb,nullzero"`
	// FallbackExternal is the value toExternal yields for unmapped input under
	// the fallback policy.
	FallbackExternal any  `json:"fallbackExternal,omitempty" bun:"fallback_external,type:jsonb,nullzero"`
	IsEnabled        bool `json:"isEnabled" bun:"is_enabled"`
}
