/*
Package rxde is a golang package for performing data extraction and parsing (and a few transformations supported along the the way) from non-structured textual data into json documents.

It intends to be as fast as possible and minimize allocations, despite using regular expressions to extract the data. Many parts of the code are intentionally inlined for that purpose.
*/
package rxde

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"

	"github.com/brunotm/rxde/rule"
)

var (
	errEmptyRules           = errors.New("empty rules")
	errRepeatedRuleName     = errors.New("repeated rule name")
	errInvalidParsersNumber = errors.New("invalid number of matches and parsers")
	errNilStartRegex        = errors.New("both StartMatch and Regex are nil")
)

// Result represents a json document and any errors from parsing and transformation
type Result struct {
	Data   []byte
	Errors []error
}

// Config for creating a parser
type Config struct {
	FindAll     bool          `json:"find_all"`     // find all ocurrences of the parser regex
	StartMatch  string        `json:"start_match"`  // start matching when matched (inclusive current line)
	StopMatch   string        `json:"stop_match"`   // stop matching when matched (terminates parsing)
	SkipMatch   string        `json:"skip_match"`   // skip lines when matched, until resume_match
	ResumeMatch string        `json:"resume_match"` // resume after skiping when matched
	Regex       string        `json:"regex"`        // regex to use when performing line oriented matching
	Rules       []rule.Config `json:"rules"`        // rules for parse and extract data
}

// Parser type. A parser has no state and is safe for concurrent use
type Parser struct {
	startMatch  *regexp.Regexp
	stopMatch   *regexp.Regexp
	skipMatch   *regexp.Regexp
	resumeMatch *regexp.Regexp
	regex       *regexp.Regexp
	rules       []*rule.Rule
	config      Config
}

// New creates a new parser with the given config
func New(config Config) (p *Parser, err error) {
	p = &Parser{}
	p.config = config

	if config.StartMatch != "" {
		p.startMatch, err = regexp.Compile(config.StartMatch)
		if err != nil {
			return nil, err
		}
	}

	if config.StopMatch != "" {
		p.stopMatch, err = regexp.Compile(config.StopMatch)
		if err != nil {
			return nil, err
		}
	}

	if config.SkipMatch != "" {
		p.skipMatch, err = regexp.Compile(config.SkipMatch)
		if err != nil {
			return nil, err
		}
	}

	if config.ResumeMatch != "" {
		p.stopMatch, err = regexp.Compile(config.ResumeMatch)
		if err != nil {
			return nil, err
		}
	}

	if config.Regex != "" {
		p.stopMatch, err = regexp.Compile(config.Regex)
		if err != nil {
			return nil, err
		}
	}

	if len(config.Rules) == 0 {
		return nil, errEmptyRules
	}

	ruleNames := make(map[string]struct{})
	for i := range config.Rules {

		// as only flat json documents are supported
		// check and error if we find a repeated rule name
		if _, ok := ruleNames[config.Rules[i].Name]; ok {
			return nil, errRepeatedRuleName
		}
		ruleNames[config.Rules[i].Name] = struct{}{}

		r, err := rule.New(config.Rules[i])
		if err != nil {
			return nil, err
		}
		p.rules = append(p.rules, r)
	}

	if p.regex == nil && p.startMatch == nil {
		return nil, errNilStartRegex
	}

	return p, nil
}

// Config returns the config usef to create this parser
func (p *Parser) Config() (c Config) {
	return p.config
}

// MarshalJSON creates a json config from this parser
func (p *Parser) MarshalJSON() (data []byte, err error) {
	return json.Marshal(&p.config)
}

// UnmarshalJSON creates a new parser from the JSON encoded configuration
func (p *Parser) UnmarshalJSON(data []byte) (err error) {
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	pp, err := New(config)
	if err != nil {
		return err
	}

	p.startMatch = pp.startMatch
	p.stopMatch = pp.stopMatch
	p.skipMatch = pp.skipMatch
	p.resumeMatch = pp.resumeMatch
	p.regex = pp.regex
	p.rules = pp.rules
	p.config = config

	return nil
}

// Processor is a callback to process each result from the parser.
// Return false to stop parsing.
type Processor func(r Result) (ok bool)

// Parse parses raw data in its own goroutine returning the parsed results in the results chan
func (p *Parser) Parse(ctx context.Context, data io.Reader) (results <-chan Result) {
	res := make(chan Result)

	go func() {
		p.ParseWith(data, func(r Result) (ok bool) {
			select {
			case res <- r:
				return true
			case <-ctx.Done():
				return false
			}
		})
		close(res)
	}()

	return res
}

// ParseWith parses raw data using the specified processor to handle parsed results
func (p *Parser) ParseWith(data io.Reader, cb Processor) {

	if p.regex == nil {
		p.parseSet(data, cb)
		return
	}

	p.parse(data, cb)
	return

}

func (p *Parser) parse(data io.Reader, cb Processor) {

	var skip bool
	var line []byte
	var match [][]byte
	var result Result
	scanner := bufio.NewScanner(data)

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			result = Result{}
			result.Errors = append(result.Errors, err)
			cb(result)
			return
		}

		line = scanner.Bytes()

		if p.stopMatch != nil && p.stopMatch.Match(line) {
			break
		}

		// Only skip sections if both skip and continue regexps are set
		if p.skipMatch != nil && p.resumeMatch != nil {
			// Set the skip flag if skipMatch is set and match the current line
			if p.skipMatch.Match(line) {
				skip = true
			}
			//  Set the skip flag if resumeMatch is set and match the current line
			if p.resumeMatch.Match(line) {
				skip = false
			}

			if skip {
				continue
			}
		}

		if p.config.FindAll {
			match = p.handleAllSubmatch(line)
		} else {
			match = p.regex.FindSubmatch(line)
		}

		if match == nil {
			continue
		}

		match = match[1:]
		if len(match) != len(p.rules) {
			result = Result{}
			result.Errors = append(result.Errors, errInvalidParsersNumber)
			if !cb(result) {
				return
			}
			continue
		}

		result = Result{}

		for r := range p.rules {

			value, _, err := p.rules[r].Parse(match[r])
			if err != nil {
				result.Errors = append(result.Errors, err)
			}

			result.Data = appendJSON(result.Data, p.rules[r].Config().Name, value)
		}

		if !cb(result) {
			return
		}
	}

}

func (p *Parser) parseSet(data io.Reader, cb Processor) {

	var skip bool
	var result Result
	scanner := bufio.NewScanner(data)

	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			result = Result{}
			result.Errors = append(result.Errors, err)
			cb(result)
			return
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		if p.stopMatch != nil && p.stopMatch.Match(line) {
			break
		}

		// Only skip sections if both skip and continue regexps are set
		if p.skipMatch != nil && p.resumeMatch != nil {
			// Set the skip flag if skipMatch is set and match the current line
			if p.skipMatch.Match(line) {
				skip = true
			}
			//  Set the skip flag if resumeMatch is set and match the current line
			if p.resumeMatch.Match(line) {
				skip = false
			}

			if skip {
				continue
			}
		}

		// If content is a match for startMatch and
		// document is valid deliver the result
		if p.startMatch.Match(line) {
			if result.Data != nil || result.Errors != nil {
				if !cb(result) {
					return
				}
			}
			result = Result{}
		}

		for r := range p.rules {

			if bytes.Index(result.Data, []byte(p.rules[r].Config().Name)) > -1 {
				continue
			}

			// Continue if we don't match this regexp
			value, ok, err := p.rules[r].Parse(line)
			if err != nil {
				result.Errors = append(result.Errors, err)
				continue
			}

			if ok {
				result.Data = appendJSON(result.Data, p.rules[r].Config().Name, value)
			}
		}
	}

	if result.Data != nil || result.Errors != nil {
		if !cb(result) {
			return
		}
	}
}

func (p *Parser) handleAllSubmatch(data []byte) (match [][]byte) {
	subs := p.regex.FindAllSubmatch(data, -1)
	if len(subs) == 0 {
		return nil
	}
	match = make([][]byte, 0, len(subs)+1)
	match = append(match, nil)
	for i := range subs {
		match = append(match, subs[i][1])
	}
	return match
}
