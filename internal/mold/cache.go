package mold

import (
	"maps"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coldsmirk/vef-framework-go/mold"
)

type tagType uint8

const (
	typeDefault tagType = iota
	typeDive
	typeKeys
	typeEndKeys
)

type structCache struct {
	lock sync.Mutex
	m    atomic.Value // map[reflect.Type]*cStruct
}

func (sc *structCache) Get(key reflect.Type) (c *cStruct, found bool) {
	c, found = sc.m.Load().(map[reflect.Type]*cStruct)[key]

	return c, found
}

func (sc *structCache) Set(key reflect.Type, value *cStruct) {
	m := sc.m.Load().(map[reflect.Type]*cStruct)
	nm := make(map[reflect.Type]*cStruct, len(m)+1)
	maps.Copy(nm, m)
	nm[key] = value
	sc.m.Store(nm)
}

type tagCache struct {
	lock sync.Mutex
	m    atomic.Value // map[string]*cTag
}

func (tc *tagCache) Get(key string) (c *cTag, found bool) {
	c, found = tc.m.Load().(map[string]*cTag)[key]

	return c, found
}

func (tc *tagCache) Set(key string, value *cTag) {
	m := tc.m.Load().(map[string]*cTag)
	nm := make(map[string]*cTag, len(m)+1)
	maps.Copy(nm, m)
	nm[key] = value
	tc.m.Store(nm)
}

type cStruct struct {
	fields map[string]*cField
	fn     mold.StructLevelFunc
}

type cField struct {
	idx   int
	cTags *cTag
}

type cTag struct {
	tag            string
	aliasTag       string
	actualAliasTag string
	hasAlias       bool
	typeof         tagType
	hasTag         bool
	fn             mold.Func
	keys           *cTag
	next           *cTag
	param          string
}

func (t *MoldTransformer) extractStructCache(current reflect.Value) (*cStruct, error) {
	t.cCache.lock.Lock()
	defer t.cCache.lock.Unlock()

	typ := current.Type()

	// could have been multiple trying to access, but once first is done this ensures struct
	// isn't parsed again.
	sc, ok := t.cCache.Get(typ)
	if ok {
		return sc, nil
	}

	sc = &cStruct{
		fields: make(map[string]*cField),
		fn:     t.structLevelFuncs[typ],
	}
	numFields := current.NumField()

	var (
		ctag  *cTag
		field reflect.StructField
		tag   string
		err   error
	)

	for i := range numFields {
		field = typ.Field(i)

		if !field.Anonymous && len(field.PkgPath) > 0 {
			continue
		}

		tag = field.Tag.Get(t.tagName)
		if tag == ignoreTag {
			continue
		}

		// NOTE: cannot use shared tag cache, because tags may be equal, but things like alias may be different
		// and so only struct level caching can be used instead of combined with Field tag caching
		if len(tag) > 0 {
			if ctag, _, err = t.parseFieldTagsRecursive(tag, field.Name, "", false); err != nil {
				return nil, err
			}
		} else {
			// even if field doesn't have validations need cTag for traversing to potential inner/nested
			// elements of the field.
			ctag = &cTag{typeof: typeDefault}
		}

		cf := &cField{
			idx:   i,
			cTags: ctag,
		}
		sc.fields[field.Name] = cf
	}

	t.cCache.Set(typ, sc)

	return sc, nil
}

func (t *MoldTransformer) parseFieldTagsRecursive(tagString, fieldName, alias string, hasAlias bool) (firstCTag, currentCTag *cTag, err error) {
	var (
		tag     string
		ok      bool
		noAlias = len(alias) == 0
		tags    = strings.Split(tagString, tagSeparator)
	)

	for i := 0; i < len(tags); i++ {
		tag = tags[i]
		if noAlias {
			alias = tag
		}

		// check map for alias and process new tags, otherwise process as usual
		if tagsVal, found := t.aliases[tag]; found {
			if i == 0 {
				firstCTag, currentCTag, err = t.parseFieldTagsRecursive(tagsVal, fieldName, tag, true)
				if err != nil {
					return firstCTag, currentCTag, err
				}
			} else {
				if currentCTag.next, currentCTag, err = t.parseFieldTagsRecursive(tagsVal, fieldName, tag, true); err != nil {
					return firstCTag, currentCTag, err
				}
			}

			continue
		}

		var prevTag tagType

		if i == 0 {
			currentCTag = &cTag{aliasTag: alias, hasAlias: hasAlias, hasTag: true}
			firstCTag = currentCTag
		} else {
			prevTag = currentCTag.typeof
			currentCTag.next = &cTag{aliasTag: alias, hasAlias: hasAlias, hasTag: true}
			currentCTag = currentCTag.next
		}

		switch tag {
		case diveTag:
			currentCTag.typeof = typeDive

			continue

		case keysTag:
			currentCTag.typeof = typeKeys

			if i == 0 || prevTag != typeDive {
				err = ErrInvalidKeysTag

				return firstCTag, currentCTag, err
			}

			currentCTag.typeof = typeKeys

			// need to pass along only keys tag
			// need to increment i to skip over the keys tags
			b := make([]byte, 0, 64)

			i++

			for ; i < len(tags); i++ {
				b = append(b, tags[i]...)
				b = append(b, ',')

				if tags[i] == endKeysTag {
					break
				}
			}

			if currentCTag.keys, _, err = t.parseFieldTagsRecursive(string(b[:len(b)-1]), fieldName, "", false); err != nil {
				return firstCTag, currentCTag, err
			}

			continue

		case endKeysTag:
			currentCTag.typeof = typeEndKeys

			// if there are more in tags then there was no keysTag defined
			// and an error should be thrown
			if i != len(tags)-1 {
				err = ErrUndefinedKeysTag
			}

			return firstCTag, currentCTag, err

		default:
			vals := strings.SplitN(tag, tagKeySeparator, 2)

			if noAlias {
				alias = vals[0]
				currentCTag.aliasTag = alias
			} else {
				currentCTag.actualAliasTag = tag
			}

			currentCTag.tag = vals[0]
			if len(currentCTag.tag) == 0 {
				err = &ErrInvalidTag{tag: currentCTag.tag, field: fieldName}

				return firstCTag, currentCTag, err
			}

			if currentCTag.fn, ok = t.transformations[currentCTag.tag]; !ok {
				err = &ErrUndefinedTag{tag: currentCTag.tag, field: fieldName}

				return firstCTag, currentCTag, err
			}

			if len(vals) > 1 {
				currentCTag.param = strings.ReplaceAll(vals[1], utf8HexComma, ",")
			}
		}
	}

	return firstCTag, currentCTag, err
}
