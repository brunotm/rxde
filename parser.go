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
	errInvalidParsersNumber = errors.New("invalid number of matches and parsers")
	errNilStartRegex        = errors.New("both StartMatch and Regex are nil")
)

// Result type
type Result struct {
	Data   []byte
	Errors []error
}

// Config type
type Config struct {
	FindAll     bool          `json:"find_all"`
	StartMatch  string        `json:"start_match"`
	StopMatch   string        `json:"stop_match"`
	SkipMatch   string        `json:"skip_match"`
	ResumeMatch string        `json:"resume_match"`
	Regex       string        `json:"regex"`
	Rules       []rule.Config `json:"rules"`
}

// Parser type
// A parser has no state and is goroutine safe
type Parser struct {
	startMatch  *regexp.Regexp
	stopMatch   *regexp.Regexp
	skipMatch   *regexp.Regexp
	resumeMatch *regexp.Regexp
	regex       *regexp.Regexp
	rules       []*rule.Rule
	config      Config
}

// New parser
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

	for i := range config.Rules {
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

// Config of this parser
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
// Return false to stop the parser
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
