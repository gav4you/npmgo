package nodeutil

import "strings"

// Translation of the [util.inspect.custom] hooks of URL and URLSearchParams
// from Node's lib/internal/url.js (v25.9.0).

// URLContext mirrors Node's URLContext (Ada component offsets), shown under
// Symbol(context) when showHidden is set.
type URLContext struct {
	Href                                string
	ProtocolEnd, UsernameEnd, HostStart float64
	HostEnd, PathnameStart, SearchStart float64
	HashStart, Port, SchemeType         float64
}

func (u *URLContext) model() *Object {
	const omitted = 4294967295
	proto := &Object{Proto: Proto{Prototype: ObjectPrototype}}
	getter := func(name string, f func() bool) Prop {
		return Prop{Key: name, Hidden: true, IsAccessor: true, Getter: func() (any, error) { return f(), nil }}
	}
	proto.Props = []Prop{
		{Key: "constructor", Hidden: true, Value: &Function{Name: "URLContext", Class: true, PrototypeObject: proto}},
		getter("hasPort", func() bool { return u.Port != omitted }),
		getter("hasSearch", func() bool { return u.SearchStart != omitted }),
		getter("hasHash", func() bool { return u.HashStart != omitted }),
	}
	return &Object{Proto: Proto{Prototype: proto}, Props: []Prop{
		{Key: "href", Value: u.Href},
		{Key: "protocol_end", Value: u.ProtocolEnd},
		{Key: "username_end", Value: u.UsernameEnd},
		{Key: "host_start", Value: u.HostStart},
		{Key: "host_end", Value: u.HostEnd},
		{Key: "pathname_start", Value: u.PathnameStart},
		{Key: "search_start", Value: u.SearchStart},
		{Key: "hash_start", Value: u.HashStart},
		{Key: "port", Value: u.Port},
		{Key: "scheme_type", Value: u.SchemeType},
	}}
}

// InspectCustom ports URL.prototype[inspect.custom].
func (u *URL) InspectCustom(depth float64, opts CustomOptions, inspect func(any, Options) string) any {
	if depth < 0 {
		return u
	}
	props := []Prop{
		{Key: "href", Value: u.Href}, {Key: "origin", Value: u.Origin},
		{Key: "protocol", Value: u.Protocol}, {Key: "username", Value: u.Username},
		{Key: "password", Value: u.Password}, {Key: "host", Value: u.Host},
		{Key: "hostname", Value: u.Hostname}, {Key: "port", Value: u.Port},
		{Key: "pathname", Value: u.Pathname}, {Key: "search", Value: u.Search},
		{Key: "searchParams", Value: u.searchParams()}, {Key: "hash", Value: u.Hash},
	}
	if opts.ShowHidden && u.Context != nil {
		d := "context"
		props = append(props, Prop{Key: Symbol{Description: &d}, Value: u.Context.model()})
	}
	return "URL " + inspect(&Object{Props: props}, opts.Options)
}

func (u *URL) searchParams() *URLSearchParams {
	if u.SearchParams != nil {
		return u.SearchParams
	}
	return &URLSearchParams{}
}

// InspectCustom ports URLSearchParams.prototype[inspect.custom].
func (p *URLSearchParams) InspectCustom(depth float64, opts CustomOptions, inspect func(any, Options) string) any {
	if depth < 0 {
		return opts.Stylize("[Object]", "special")
	}
	inner := opts.Options
	inner.Depth = depth - 1
	output := make([]string, len(p.Names))
	for i := range p.Names {
		output[i] = inspect(p.Names[i], inner) + " => " + inspect(p.Values[i], inner)
	}
	const sep = ", "
	length := -len(sep)
	for _, o := range output {
		length += utf16Len(removeColors(o)) + len(sep)
	}
	switch {
	case float64(length) > opts.BreakLength:
		return "URLSearchParams {\n  " + strings.Join(output, ",\n  ") + " }"
	case len(output) > 0:
		return "URLSearchParams { " + strings.Join(output, sep) + " }"
	}
	return "URLSearchParams {}"
}

// JSModel: with customInspect off, URLSearchParams has no own keys.
func (p *URLSearchParams) JSModel() any {
	return &Object{Proto: Proto{Constructor: "URLSearchParams"}, Tag: "URLSearchParams"}
}

// serialize ports URLSearchParams.prototype.toString: the
// application/x-www-form-urlencoded serializer.
func (p *URLSearchParams) serialize() string {
	var b strings.Builder
	for i := range p.Names {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(formURLEncode(p.Names[i]))
		b.WriteByte('=')
		b.WriteString(formURLEncode(p.Values[i]))
	}
	return b.String()
}

func formURLEncode(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case c == '*' || c == '-' || c == '.' || c == '_' ||
			(c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z'):
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}
