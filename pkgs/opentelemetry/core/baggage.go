// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/core 2.11.0 (build/src/baggage/constants.js,
// baggage/utils.js and baggage/propagation/W3CBaggagePropagator.js).

package core

import (
	"strings"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/pkgs/opentelemetry/api"
)

const (
	baggageKeyPairSeparator    = "="
	baggagePropertiesSeparator = ";"
	baggageItemsSeparator      = ","
	// baggageHeader is the name of the HTTP header that carries the baggage.
	baggageHeader = "baggage"
	// baggageMaxNameValuePairs is the most name-value pairs the W3C spec allows.
	baggageMaxNameValuePairs = 180
	// baggageMaxPerNameValuePairs is the most bytes (UTF-16 units here) of one name-value pair.
	baggageMaxPerNameValuePairs = 4096
	// baggageMaxTotalLength is the most bytes (UTF-16 units here) of all name-value pairs.
	baggageMaxTotalLength = 8192
)

// serializeKeyPairs joins the pairs with commas, skipping each pair that would take the header above 8192 UTF-16
// units (a later, shorter pair may still fit).
func serializeKeyPairs(keyPairs []string) string {
	hValue := ""
	for _, current := range keyPairs {
		value := hValue
		if hValue != "" {
			value += baggageItemsSeparator
		}
		value += current
		if jsvalue.Len16(value) <= baggageMaxTotalLength {
			hValue = value
		}
	}
	return hValue
}

// getKeyPairs renders each entry as encodeURIComponent(key)=encodeURIComponent(value), followed by ;metadata when
// there is metadata (not encoded: that is the metadata's job). A key or value that is not valid UTF-8 (a lone
// surrogate in JavaScript) panics with a URIError.
func getKeyPairs(baggage api.Baggage) []string {
	entries := baggage.GetAllEntries()
	out := make([]string, len(entries))
	for i, e := range entries {
		key, err := jsvalue.EncodeURIComponent(e.Key)
		if err != nil {
			panic(err)
		}
		value, err := jsvalue.EncodeURIComponent(e.Entry.Value)
		if err != nil {
			panic(err)
		}
		entry := key + "=" + value
		if e.Entry.Metadata != nil {
			entry += baggagePropertiesSeparator + e.Entry.Metadata.ToString()
		}
		out[i] = entry
	}
	return out
}

// keyPair is what parsePairKeyValue returns.
type keyPair struct {
	key, value string
	metadata   api.BaggageEntryMetadata
}

// parsePairKeyValue parses key=value[;metadata], with whitespace around the key and the value trimmed (JavaScript's
// trim) and both percent-decoded; nil when the entry is empty, has no key or value, or does not decode.
func parsePairKeyValue(entry string) *keyPair {
	if entry == "" {
		return nil
	}
	metadataSeparatorIndex := strings.Index(entry, baggagePropertiesSeparator)
	keyPairPart := entry
	if metadataSeparatorIndex != -1 {
		keyPairPart = entry[:metadataSeparatorIndex]
	}
	separatorIndex := strings.Index(keyPairPart, baggageKeyPairSeparator)
	if separatorIndex <= 0 {
		return nil
	}
	rawKey := jsvalue.Trim(keyPairPart[:separatorIndex])
	rawValue := jsvalue.Trim(keyPairPart[separatorIndex+1:])
	if rawKey == "" || rawValue == "" {
		return nil
	}
	key, err := jsvalue.DecodeURIComponent(rawKey)
	if err != nil {
		return nil
	}
	value, err := jsvalue.DecodeURIComponent(rawValue)
	if err != nil {
		return nil
	}
	var metadata api.BaggageEntryMetadata
	if metadataSeparatorIndex != -1 && metadataSeparatorIndex < len(entry)-1 {
		metadata = api.BaggageEntryMetadataFromString(entry[metadataSeparatorIndex+1:])
	}
	return &keyPair{key, value, metadata}
}

// baggageRecord is the object W3CBaggagePropagator.extract assigns the parsed entries to.
type baggageRecord []api.BaggageEntryPair

// parseBaggageHeaderString parses one header value into the record, within the limits: entries longer than 4096
// UTF-16 units are skipped, at most 180 entries are taken and parsing stops at the entry that would take the total
// (each entry after the first counting a comma) above 8192. It returns the updated count and total.
//
// value is what the getter returned. A string is parsed. JavaScript reads value.length first: null and undefined
// throw (a TypeError), a non-empty array throws when substring is called on it, and any other value has no length
// and adds nothing. (An array longer than 4096 elements is not modelled.)
func parseBaggageHeaderString(value any, record *baggageRecord, count, totalSize int) (int, int) {
	s, ok := value.(string)
	if !ok {
		switch {
		case value == nil:
			panic(&jsvalue.TypeError{Message: "Cannot read properties of null (reading 'length')"})
		case jsvalue.IsUndefined(value):
			panic(&jsvalue.TypeError{Message: "Cannot read properties of undefined (reading 'length')"})
		case jsvalue.IsArray(value) && len(jsvalue.Items(value)) > 0:
			panic(&jsvalue.TypeError{Message: "value.substring is not a function"})
		}
		return count, totalSize
	}
	// Indices are byte offsets of s; the separators are ASCII, so the substrings are JavaScript's, and lengths are
	// counted in UTF-16 units as JavaScript counts them.
	start := 0
	for start < len(s) && count < baggageMaxNameValuePairs {
		end := strings.Index(s[start:], baggageItemsSeparator)
		if end != -1 {
			end += start
		}
		entryEnd := end
		if end == -1 {
			entryEnd = len(s)
		}
		entryLength := jsvalue.Len16(s[start:entryEnd])
		if entryLength <= baggageMaxPerNameValuePairs {
			if kp := parsePairKeyValue(s[start:entryEnd]); kp != nil {
				entrySize := entryLength
				if count != 0 {
					entrySize++
				}
				if totalSize+entrySize > baggageMaxTotalLength {
					break
				}
				*record = append(*record, api.BaggageEntryPair{Key: kp.key, Entry: api.BaggageEntry{Value: kp.value, Metadata: kp.metadata}})
				count++
				totalSize += entrySize
			}
		}
		if end == -1 {
			break
		}
		start = end + 1
	}
	return count, totalSize
}

// ParseKeyPairsIntoRecord parses a baggage header (without metadata) into a record of key: value, in JavaScript
// object order (array-index keys first; a repeated key keeps its first place and its last value; __proto__ is
// dropped, as the assignment sets the prototype). A value that is not a string, or is empty, gives an empty record.
func ParseKeyPairsIntoRecord(value any) *jsvalue.Object {
	result := &jsvalue.Object{}
	s, ok := value.(string)
	if !ok || s == "" {
		return result
	}
	for _, entry := range strings.Split(s, baggageItemsSeparator) {
		if kp := parsePairKeyValue(entry); kp != nil && kp.value != "" {
			result.Put(kp.key, kp.value)
		}
	}
	return result
}

// W3CBaggagePropagator propagates baggage in the W3C baggage header (https://w3c.github.io/baggage/).
type W3CBaggagePropagator struct{}

// NewW3CBaggagePropagator is new W3CBaggagePropagator().
func NewW3CBaggagePropagator() *W3CBaggagePropagator { return &W3CBaggagePropagator{} }

// Inject writes the context's baggage, unless tracing is suppressed: the first 180 entries of at most 4096 UTF-16
// units each, up to 8192 units in all. An empty header is not written.
func (p *W3CBaggagePropagator) Inject(context api.Context, carrier any, setter api.TextMapSetter) {
	baggage := api.Propagation.GetBaggage(context)
	if baggage == nil || IsTracingSuppressed(context) {
		return
	}
	var keyPairs []string
	for _, pair := range getKeyPairs(baggage) {
		if jsvalue.Len16(pair) <= baggageMaxPerNameValuePairs {
			keyPairs = append(keyPairs, pair)
		}
	}
	if len(keyPairs) > baggageMaxNameValuePairs {
		keyPairs = keyPairs[:baggageMaxNameValuePairs]
	}
	if headerValue := serializeKeyPairs(keyPairs); headerValue != "" {
		setter.Set(carrier, baggageHeader, headerValue)
	}
}

// Extract returns context with the baggage of the carrier's baggage header (each element of a list in turn, within
// the limits shared by all of them), or context itself when there is no (truthy) header or no entry in it.
func (p *W3CBaggagePropagator) Extract(context api.Context, carrier any, getter api.TextMapGetter) api.Context {
	headerValue := getter.Get(carrier, baggageHeader)
	if !jsvalue.Truthy(headerValue) {
		return context
	}
	var record baggageRecord
	count, totalSize := 0, 0
	if jsvalue.IsArray(headerValue) {
		for _, v := range jsvalue.Items(headerValue) {
			count, totalSize = parseBaggageHeaderString(v, &record, count, totalSize)
		}
	} else {
		count, _ = parseBaggageHeaderString(headerValue, &record, count, totalSize)
	}
	if count == 0 {
		return context
	}
	return api.Propagation.SetBaggage(context, api.Propagation.CreateBaggage(record...))
}

// Fields returns ["baggage"].
func (p *W3CBaggagePropagator) Fields() []string { return []string{baggageHeader} }
