package rule

import (
	"bytes"
	"testing"
)

var (
	ruleNumberJSON = []byte(`{
		"name": "jsontestval",
		"type": "number",
		"regex": "([-+]?\\d*\\.?\\d+)"
		}`)
	ruleNumberData   = []byte(`aaa ccc 3.8876 ccc`)
	ruleNumberExpect = []byte(`3.8876`)
)

var cases = []struct {
	config Config
	data   []byte
	expect []byte
}{
	{Config{Name: "int", Type: Int,
		Regex: `(\d+)`}, []byte(`aaaa12344.ffff`), []byte(`12344`)},

	{Config{Name: "uint", Type: Uint,
		Regex: `(\d+)`}, []byte(`aaaa12344.ffff`), []byte(`12344`)},

	{Config{Name: "float", Type: Float,
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`124.545`)},

	{Config{Name: "number", Type: Number,
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`124.545`)},

	{Config{Name: "string", Type: String,
		Regex: `\s(\w+)\s`}, []byte(`545 ffff -f`), []byte(`"ffff"`)},

	{Config{Name: "bool", Type: Bool,
		Regex: `\s(\w+)\s`}, []byte(`545 true -f`), []byte(`true`)},

	{Config{Name: "duration_s_to_ms", Type: Duration, From: "s", To: "ms",
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`124545`)},

	{Config{Name: "duration_h_to_min", Type: Duration, From: "h", To: "min",
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`7472.7`)},

	{Config{Name: "duration_h_to_ns", Type: Duration, From: "h", To: "ns",
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`448362000000000`)},

	{Config{Name: "duration_m_to_h", Type: Duration, From: "m", To: "h",
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`2.07575`)},

	{Config{Name: "duration_to_string", Type: Duration, From: "m", To: "string",
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa124.545 ffff`), []byte(`"2h4m32.7s"`)},

	{Config{Name: "duration_to_ms", Type: Duration, To: "ms",
		Regex: `([-+]?\d*\.?\d+)`}, []byte(`aaaa1.545 ffff`), []byte(`1545`)},

	{Config{Name: "time_unix_to_iso8601", Type: Time, From: "unix", To: "iso8601",
		Regex: `(\d+)`}, []byte(`time:1537335984`), []byte(`"2018-09-19T06:46:24.000+0100"`)},

	{Config{Name: "time_iso8601_to_unix", Type: Time, From: "iso8601", To: "unix",
		Regex: `(.*)`}, []byte(`2018-09-19T06:46:24.000+0100`), []byte(`1537335984`)},

	{Config{Name: "time_iso8601_to_unixnano", Type: Time, From: "iso8601", To: "unix_nano",
		Regex: `(.*)`}, []byte(`2018-09-19T06:46:24.000+0100`), []byte(`1537335984000000000`)},

	{Config{Name: "time_custom_to_rfc3339", Type: Time, From: "Mon Jan 02 15:04:05 2006", To: "rfc3339",
		Regex: `(.*)`}, []byte(`Mon Sep 21 23:09:05 2018`), []byte(`"2018-09-21T23:09:05Z"`)},

	{Config{Name: "time_unix_to_rfc3339", Type: Time, From: "unix", To: "rfc3339",
		Regex: `(\d+)`}, []byte(`time:1537335984`), []byte(`"2018-09-19T06:46:24+01:00"`)},

	{Config{Name: "time_custom_to_custom", Type: Time, From: "unix", To: "Mon Jan 02 15:04:05 2006",
		Regex: `(\d+)`}, []byte(`time:1537335984`), []byte(`"Wed Sep 19 06:46:24 2018"`)},

	{Config{Name: "datasize_bytes_to_kib", Type: DataSize, To: "kib",
		Regex: `(\d+\w*)`}, []byte(`datasize:1mib`), []byte(`1024`)},
	{Config{Name: "datasize_bytes_to_kib_explicit", Type: DataSize, From: "mib", To: "kib",
		Regex: `(\d+\w*)`}, []byte(`datasize:1mib`), []byte(`1024`)},
}

func TestUnmarshal(t *testing.T) {
	v := &Rule{}
	err := v.UnmarshalJSON(ruleNumberJSON)

	if err != nil {
		t.Fatal(err)
	}

	value, ok, err := v.Parse(ruleNumberData)
	if !ok || err != nil {
		t.Fatal(ok, err)
	}

	if bytes.Compare(value, ruleNumberExpect) != 0 {
		t.Fatal("not equal: ", bytesToString(value), bytesToString(ruleNumberExpect))
	}

	t.Log("value: ", bytesToString(value))
}

func TestParse(t *testing.T) {
	for _, testCase := range cases {
		t.Run(testCase.config.Name, func(t *testing.T) {
			v, err := New(testCase.config)
			if err != nil {
				t.Fatal(err)
			}

			value, ok, err := v.Parse(testCase.data)
			if !ok || err != nil {
				t.Fatal(ok, err)
			}

			if bytes.Compare(value, testCase.expect) != 0 {
				t.Fatal("not equal: ", bytesToString(value), bytesToString(testCase.expect))
			}

			t.Log("value: ", bytesToString(value))
		})
	}
}

var v []byte
var m bool
var err error

func BenchmarkTestCase(b *testing.B) {

	values := []*Rule{}
	for _, testCase := range cases {
		v, err := New(testCase.config)
		if err != nil {
			b.Fatal(err)
		}
		values = append(values, v)
	}

	for i := range values {
		b.Run(values[i].config.Name, func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				v, m, err = values[i].Parse(cases[i].data)
			}
		})
	}
}
