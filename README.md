# rxde
[![Build Status](https://travis-ci.org/brunotm/rxde.svg?branch=master)](https://travis-ci.org/brunotm/rxde) [![Go Report Card](https://goreportcard.com/badge/github.com/brunotm/rxde)](https://goreportcard.com/report/github.com/brunotm/rxde)
--------------------
rxde is a golang package for performing data extraction and parsing (and a few transformations supported along the the way) from non-structured textual data into json documents.

It intends to be as fast as possible and minimize allocations, despite using regular expressions to extract the data. Many parts of the code are intentionally inlined for that purpose.

## Example
```go
package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/brunotm/rxde"
	"github.com/brunotm/rxde/rule"
)

var (
	data = []byte(`
	197960064 K total memory
    112675296 K used memory
    131243072 K active memory
     54145440 K inactive memory
       847940 K free memory
       386224 K buffer memory
     84050592 K swap cache
     25165820 K total swap
     14636732 K used swap
     10529088 K free swap
	 1539723059 timestamp
	 132m		duration
	`)
)

func main() {
	parser, err := rxde.New(
		rxde.Config{
			StartMatch: "total memory",
			Rules: []rule.Config{
				{
					Name:  "total_memory_gb",
					Type:  "datasize",
					To:    "gb",
					Regex: "(\\d+ \\w) total memory",
				},
				{
					Name:  "used_memory_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) used memory",
				},
				{
					Name:  "active_memory_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) active memory",
				},
				{
					Name:  "inactive_memory_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) inactive memory",
				},
				{
					Name:  "free_memory_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) free memory",
				},
				{
					Name:  "buffer_memory_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) buffer memory",
				},
				{
					Name:  "swap_cache_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) swap cache",
				},
				{
					Name:  "total_swap_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) total swap",
				},
				{
					Name:  "used_swap_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) used swap",
				},
				{
					Name:  "free_swap_mb",
					Type:  "datasize",
					To:    "mb",
					Regex: "(\\d+ \\w) free swap",
				},
				{
					Name:  "timestamp",
					Type:  "time",
					From:  "unix",
					To:    "iso8601",
					Regex: "(\\d+) timestamp",
				},
				{
					Name:  "duration_hours",
					Type:  "duration",
					To:    "hour",
					Regex: "(\\w+)\\s+duration",
				},
			},
		},
	)

	if err != nil {
		panic(err)
	}

	// Using the lower level callback api
	parser.ParseWith(bytes.NewReader(data), func(r rxde.Result) bool {
		fmt.Println(r.Errors)
		fmt.Println(string(r.Data))
		return true
	})

	// Or the channel api
	for r := range parser.Parse(context.Background(), bytes.NewReader(data)) {
		fmt.Println(r.Errors)
		fmt.Println(string(r.Data))
	}

}
```

---------------------------
Written by Bruno Moura <brunotm@gmail.com>