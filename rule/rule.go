package rule

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unsafe"
)

type (
	// Type to parse to
	Type string
)

const (
	Int      Type = "int"
	Uint     Type = "uint"
	Float    Type = "float"
	Number   Type = "number"
	String   Type = "string"
	Bool     Type = "bool"
	Time     Type = "time"
	Duration Type = "duration"
	DataSize Type = "datasize"
	// DataRate Type = "datarate"

	// Decimal
	bytE float64 = 1
	kb   float64 = bytE * 1000
	mb   float64 = kb * 1000
	gb   float64 = mb * 1000
	tb   float64 = gb * 1000
	pb   float64 = tb * 1000

	// Binary
	kib float64 = bytE * 1024
	mib float64 = kib * 1024
	gib float64 = mib * 1024
	tib float64 = gib * 1024
	pib float64 = tib * 1024

	hex     = "0123456789abcdef"
	iso8601 = "2006-01-02T15:04:05.000Z0700"
)

var (
	rexUnit   = regexp.MustCompile(`([-+]?\d*\.?\d+)\s*([a-z,A-Z]*)`)
	dataUnits = map[string]float64{
		"":          bytE,
		"b":         bytE,
		"byte":      bytE,
		"k":         kb,
		"kb":        kb,
		"kilo":      kb,
		"kilobyte":  kb,
		"kilobytes": kb,
		"m":         mb,
		"mb":        mb,
		"mega":      mb,
		"megabyte":  mb,
		"megabytes": mb,
		"g":         gb,
		"gb":        gb,
		"giga":      gb,
		"gigabyte":  gb,
		"gigabytes": gb,
		"t":         tb,
		"tb":        tb,
		"tera":      tb,
		"terabyte":  tb,
		"terabytes": tb,
		"p":         pb,
		"pb":        pb,
		"peta":      pb,
		"petabyte":  pb,
		"petabytes": pb,
		"ki":        kib,
		"kib":       kib,
		"kibibyte":  kib,
		"kibibytes": kib,
		"mi":        mib,
		"mib":       mib,
		"mebibyte":  mib,
		"mebibytes": mib,
		"gi":        gib,
		"gib":       gib,
		"gibibyte":  gib,
		"gibibytes": gib,
		"ti":        tib,
		"tib":       tib,
		"tebibyte":  tib,
		"tebibytes": tib,
		"pi":        pib,
		"pib":       pib,
		"pebibyte":  pib,
		"pebibytes": pib,
	}

	errNoRuleName       = errors.New("empty name value name")
	errInvalidType      = errors.New("invalid value type")
	errInvalidMatchNum  = errors.New("invalid number of match groups in expression")
	errInvalidDstFormat = errors.New("invalid destination format")
	errInvalidSrcFormat = errors.New("invalid source format")
	errNoMatch          = errors.New("no match")
)

// Config rule
type Config struct {
	Name  string `json:"name"`  // Rule name
	Type  Type   `json:"type"`  // Type to parse to
	From  string `json:"from"`  // Optional format or unit to parse from
	To    string `json:"to"`    // Optional format or unit to parse to
	Regex string `json:"regex"` // Optional regexp used to extract data
}

// Rule to parse the given []byte string into the specified JSON serialization for Type.
// If a Regexp with a match group is specified, will be used to extract a subset of the given data.
// Units, origin and destination formats can be specified using the From/To parameters.
// A rule has no state and is goroutine safe
type Rule struct {
	regex  *regexp.Regexp
	config Config
}

// New rule
func New(config Config) (rule *Rule, err error) {
	rule = &Rule{}
	if config.Regex != "" {
		if rule.regex, err = regexp.Compile(config.Regex); err != nil {
			return rule, err
		}

		if rule.regex.NumSubexp() != 1 {
			return nil, errInvalidMatchNum
		}
	}

	if config.Name == "" {
		return nil, errNoRuleName
	}

	if config.Type == "" {
		return nil, errInvalidType
	}

	rule.config = config
	return rule, err
}

// MustNew is like New but panics on error
func MustNew(config Config) (rule *Rule) {
	var err error
	rule, err = New(config)
	if err != nil {
		panic(err)
	}
	return rule
}

// Config of this rule
func (r *Rule) Config() (c Config) {
	return r.config
}

// MarshalJSON creates a json config from this rule
func (r *Rule) MarshalJSON() (data []byte, err error) {
	return json.Marshal(&r.config)
}

// UnmarshalJSON configures this value from a json config
func (r *Rule) UnmarshalJSON(data []byte) (err error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	rr, err := New(config)
	if err != nil {
		return err
	}

	r.config = config
	r.regex = rr.regex

	return nil
}

// Parse and transform the given data into the specified JSON serialization for Type.
func (r *Rule) Parse(b []byte) (value []byte, matched bool, err error) {

	// As we wont mutate the input avoid unecessary allocations
	s := bytesToString(b)

	// Extract data with the provided regex if defined
	if r.regex != nil {
		match := r.regex.FindStringSubmatch(s)
		if match == nil {
			return nil, false, nil
		}

		s = match[1]
	}

	if len(s) == 0 {
		return nil, true, nil
	}

	switch r.config.Type {

	case String:
		value = jsonString(s)

	case Int:
		var i int64
		i, err = strconv.ParseInt(s, 10, 64)
		value = strconv.AppendInt(value, i, 10)

	case Uint:
		var u uint64
		u, err = strconv.ParseUint(s, 10, 64)
		value = strconv.AppendUint(value, u, 10)

	case Float, Number:
		var f float64
		f, err = strconv.ParseFloat(s, 64)
		value = strconv.AppendFloat(value, f, 'f', -1, 64)

	case Bool:
		var bl bool
		bl, err = strconv.ParseBool(s)
		value = strconv.AppendBool(value, bl)

	case Time:
		value, err = r.parseTime(s)

	case Duration:
		value, err = r.parseDuration(s)

	case DataSize:
		value, err = r.parseDataSize(s)

	default:
		err = errInvalidType
	}

	if err != nil {
		err = fmt.Errorf("rule %s, input: %s, error: %s", r.config.Name, s, err)
	}

	return value, true, err
}

// parseDuration parses a string representation of duration into a specified time unit or in a time.Duration
func (r *Rule) parseDuration(s string) (value []byte, err error) {

	s = strings.ToLower(s)

	// Use From if no unit available or use seconds as default
	if !unicode.IsLetter(rune(s[len(s)-1])) {
		if r.config.From == "" {
			s = s + "s"
		} else {
			s = s + r.config.From
		}
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, err
	}

	switch r.config.To {

	case "nanoseconds", "nanosecond", "nano", "ns":
		value = strconv.AppendInt(value, d.Nanoseconds(), 10)

	case "milliseconds", "millisecond", "milli", "ms":
		value = strconv.AppendInt(value, d.Nanoseconds()/int64(time.Millisecond), 10)

	case "seconds", "second", "sec", "s":
		value = strconv.AppendFloat(value, d.Seconds(), 'f', -1, 64)

	case "minutes", "minute", "min", "m":
		value = strconv.AppendFloat(value, d.Minutes(), 'f', -1, 64)

	case "hours", "hour", "h":
		value = strconv.AppendFloat(value, d.Hours(), 'f', -1, 64)

	case "string":
		value = make([]byte, 0, 32)
		value = append(value, '"')
		value = append(value, d.String()...)
		value = append(value, '"')

	default:
		err = errInvalidDstFormat
	}

	return value, err
}

// parseTime parses a string representation of time from the specified format into a specified format or in a time.Time
func (r *Rule) parseTime(s string) (value []byte, err error) {

	var t time.Time

	switch r.config.From {

	case "unix":
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		t = time.Unix(i, 0)

	case "unix_nano":
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		t = time.Unix(0, i)

	case "unix_milli":
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		t = time.Unix(0, i*1000000)

	case "rfc3339":
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, err
		}

	case "rfc3339nano":
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return nil, err
		}

	case "iso8601":
		t, err = time.Parse(iso8601, s)
		if err != nil {
			return nil, err
		}

	default:
		t, err = time.Parse(r.config.From, s)
		if err != nil {
			return nil, err
		}
	}

	switch r.config.To {

	case "unix":
		value = strconv.AppendInt(value, t.Unix(), 10)

	case "unix_milli":
		value = strconv.AppendInt(value, t.UnixNano()/int64(time.Millisecond), 10)

	case "unix_nano":
		value = strconv.AppendInt(value, t.UnixNano(), 10)

	case "rfc3339":
		value = timeAppend(t, time.RFC3339)

	case "rfc3339nano", "string":
		value = timeAppend(t, time.RFC3339Nano)

	case "iso8601":
		value = timeAppend(t, iso8601)

	case "":
		err = errInvalidDstFormat

	default:
		value = timeAppend(t, r.config.To)
	}

	return value, err
}

func timeAppend(t time.Time, l string) (value []byte) {
	value = make([]byte, 0, len(l)+2)
	value = append(value, '"')
	value = t.AppendFormat(value, l)
	value = append(value, '"')
	return value
}

// parseDataSize parses a digital unit string representation into a float64 in
// bytes or any other unit format
func (r *Rule) parseDataSize(s string) (value []byte, err error) {

	match := rexUnit.FindStringSubmatch(s)
	if match == nil {
		return nil, errNoMatch
	}

	val, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil, err
	}

	u := r.config.From
	if u == "" {
		u = strings.ToLower(match[2])
	}

	unit, ok := dataUnits[u]
	if !ok {
		return nil, errInvalidSrcFormat
	}
	val = val * unit

	// Convert to the specified unit
	unit, ok = dataUnits[r.config.To]
	if !ok {
		return nil, errInvalidDstFormat
	}

	value = strconv.AppendFloat(value, val/unit, 'f', -1, 64)
	return value, nil
}

func jsonString(b string) (value []byte) {
	l := len(b)
	value = append(value, '"')

	for i := 0; i < l; i++ {
		c := b[i]
		if c >= 0x20 && c != '\\' && c != '"' {
			value = append(value, c)
			continue
		}
		switch c {
		case '"', '\\':
			value = append(value, '\\', c)
		case '\n':
			value = append(value, '\\', 'n')
		case '\f':
			value = append(value, '\\', 'f')
		case '\b':
			value = append(value, '\\', 'b')
		case '\r':
			value = append(value, '\\', 'r')
		case '\t':
			value = append(value, '\\', 't')
		default:
			value = append(value, `\u00`...)
			value = append(value, hex[c>>4], hex[c&0xF])
		}
		continue
	}

	value = append(value, '"')
	return value
}

func bytesToString(b []byte) (s string) {
	return *(*string)(unsafe.Pointer(&b))
}

func stringToByte(s string) []byte {
	strHeader := (*reflect.StringHeader)(unsafe.Pointer(&s))

	var b []byte
	byteHeader := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	byteHeader.Data = strHeader.Data

	l := len(s)
	byteHeader.Len = l
	byteHeader.Cap = l
	return b
}
