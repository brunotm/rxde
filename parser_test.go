package rxde

import (
	"bytes"
	"context"
	"testing"
)

var (
	parserJSON = []byte(`{
		"find_all": false,
		"start_match": "start",
		"stop_match": "",
		"skip_match": "",
		"resume_match": "",
		"regex": "",
		"rules": [
			{
				"name": "float",
				"type": "float",
				"regex": "([-+]?\\d*\\.?\\d+)"
			},
			{
				"name": "bool",
				"type": "bool",
				"regex": "\\sbool(\\w+)$"
			},
			{
				"name": "duration",
				"type": "duration",
				"to": "string",
				"regex": "([-+]?\\d*\\.?\\d+) ffff"
			}
		]
	}`)

	parserJSONdata = []byte(`start
							9.57889
							boolfalse
							aaaa124.545 ffff

							start
							2245.6
							booltrue
							aaaa66.545 ffff`)
	parserJSONexpect = []byte(`{"float":2245.6,"bool":true,"duration":"1m6.545s"}`)
)

var result Result

func TestMarshalUnmarshal(t *testing.T) {
	p := &Parser{}
	err := p.UnmarshalJSON(parserJSON)
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseWith(t *testing.T) {
	p := &Parser{}
	err := p.UnmarshalJSON(parserJSON)
	if err != nil {
		t.Fatal(err)
	}

	p.ParseWith(bytes.NewReader(parserJSONdata), func(r Result) (ok bool) {
		if r.Errors != nil {
			t.Fatal(r.Errors)
			return false
		}

		// t.Log("RESULT ", string(r.Data))
		result = r
		return true
	})

	if bytes.Compare(result.Data, parserJSONexpect) != 0 {
		t.Fatal("not equal: ", string(result.Data), string(parserJSONexpect))
	}
}

func BenchmarkParse(b *testing.B) {

	p := &Parser{}
	err := p.UnmarshalJSON(parserJSON)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		for r := range p.Parse(context.Background(), bytes.NewReader(parserJSONdata)) {
			result = r
		}
	}

}

func BenchmarkParseWith(b *testing.B) {

	p := &Parser{}
	err := p.UnmarshalJSON(parserJSON)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		p.ParseWith(bytes.NewReader(parserJSONdata), func(r Result) (ok bool) {
			result = r
			return true
		})
	}

}
