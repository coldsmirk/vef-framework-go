package strategy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coldsmirk/vef-framework-go/approval"
)

// describer is what every kind-keyed resolver has in common: it names the kind
// it handles and describes how the designer should offer it. One constraint
// lets assignee, CC, and initiator resolvers share the whole registration
// pipeline instead of repeating it three times.
type describer[K ~string] interface {
	Describe() approval.KindDescriptor[K]
}

// overlayKinds merges host-registered resolvers onto the framework built-ins.
// A host resolver whose kind matches a built-in replaces it **in place**, so
// the designer's option order does not shift when an application overrides one
// kind; any other host resolver is appended, in ascending kind order.
//
// The returned slice is the designer-facing order and the map is the
// resolution index; both describe the same set.
//
// Every descriptor is validated here rather than at first use: a host that
// registers a blank kind or an out-of-enum selection mode gets a boot failure
// naming it, instead of a flow that saves and then cannot be deployed.
func overlayKinds[K ~string, R describer[K]](builtins, hosts []R) ([]R, map[K]R, error) {
	sortedHosts, err := orderHostResolvers[K](hosts)
	if err != nil {
		return nil, nil, err
	}

	ordered := make([]R, 0, len(builtins)+len(sortedHosts))
	position := make(map[K]int, len(builtins)+len(sortedHosts))

	for _, resolver := range slices.Concat(builtins, sortedHosts) {
		if any(resolver) == nil {
			return nil, nil, errNilResolver
		}

		descriptor := resolver.Describe()
		if err := descriptor.Validate(); err != nil {
			return nil, nil, fmt.Errorf("%w: %w", errInvalidKindDescriptor, err)
		}

		if i, ok := position[descriptor.Kind]; ok {
			ordered[i] = resolver

			continue
		}

		position[descriptor.Kind] = len(ordered)
		ordered = append(ordered, resolver)
	}

	index := make(map[K]R, len(ordered))
	for _, resolver := range ordered {
		index[resolver.Describe().Kind] = resolver
	}

	return ordered, index, nil
}

// hostEntry pairs a host resolver with the kind it claims, so ordering and the
// duplicate check read the descriptor once instead of once per comparison.
type hostEntry[K ~string, R any] struct {
	kind     K
	resolver R
}

// orderHostResolvers puts host registrations into ascending kind order and
// refuses a kind claimed twice.
//
// Both rules exist because an fx value group is delivered in a randomized
// order, which is not a contract a host can build on. Appending in arrival
// order would reshuffle the designer's dropdown on every restart, and
// "last registration wins" among two resolvers claiming one kind would be a
// per-boot coin flip over which one actually resolves work — a
// configuration mistake that must fail at boot naming the kind, not resolve
// differently each time. Overriding a *built-in* is unaffected: that is one
// host registration whose kind matches a framework one, handled by the
// in-place replacement above.
func orderHostResolvers[K ~string, R describer[K]](hosts []R) ([]R, error) {
	entries := make([]hostEntry[K, R], 0, len(hosts))

	for _, resolver := range hosts {
		if any(resolver) == nil {
			return nil, errNilResolver
		}

		entries = append(entries, hostEntry[K, R]{kind: resolver.Describe().Kind, resolver: resolver})
	}

	slices.SortFunc(entries, func(a, b hostEntry[K, R]) int {
		return strings.Compare(string(a.kind), string(b.kind))
	})

	ordered := make([]R, 0, len(entries))

	for i, entry := range entries {
		if i > 0 && entry.kind == entries[i-1].kind {
			return nil, fmt.Errorf("%w: %s", errDuplicateHostKind, entry.kind)
		}

		ordered = append(ordered, entry.resolver)
	}

	return ordered, nil
}

// describeAll projects resolvers to their descriptors, preserving order — the
// designer metadata for one kind vocabulary.
func describeAll[K ~string, R describer[K]](resolvers []R) []approval.KindDescriptor[K] {
	descriptors := make([]approval.KindDescriptor[K], 0, len(resolvers))
	for _, resolver := range resolvers {
		descriptors = append(descriptors, resolver.Describe())
	}

	return descriptors
}

// requireKinds reports the first expected kind with no registered resolver.
// Built-ins are guaranteed to exist, so a miss means the framework's own
// registration list and its enum drifted apart.
func requireKinds[K ~string, R any](index map[K]R, expected []K, missing error) error {
	for _, kind := range expected {
		if _, ok := index[kind]; !ok {
			return fmt.Errorf("%w: %s", missing, kind)
		}
	}

	return nil
}
