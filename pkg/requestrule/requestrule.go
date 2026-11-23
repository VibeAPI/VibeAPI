// Package requestrule holds the request rules that channels with request rules
// turned on check before relaying: which client requests a rule matches, and
// the error the gateway answers a matched request with instead of relaying it.
package requestrule

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/tidwall/gjson"
)

// Endpoints name the client-facing APIs a rule applies to. Rules check the
// request as the client sent it, so field paths follow that API's format.
const (
	EndpointClaudeMessages  = "claude_messages"
	EndpointOpenAIChat      = "openai_chat"
	EndpointOpenAIResponses = "openai_responses"
	EndpointGemini          = "gemini"
)

const (
	SourceJSON   = "json"
	SourceBody   = "body"
	SourceHeader = "header"

	OpRegex     = "regex"
	OpNotRegex  = "not_regex"
	OpExists    = "exists"
	OpNotExists = "not_exists"

	MatchAll = "all"
	MatchAny = "any"

	DefaultErrorCode = string(types.ErrorCodeRequestRuleBlocked)
)

const (
	maxRules         = 100
	maxConditions    = 10
	maxPatternLength = 500
	maxPathLength    = 200
	maxNameRunes     = 60
	maxDescRunes     = 300
	maxMessageRunes  = 500
	maxValueRunes    = 80
	// Full-body conditions scan only the start of the body, so a request of
	// tens of megabytes is not scanned once per rule.
	maxBodyScanBytes = 1 << 20
)

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	codePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	endpoints   = []string{EndpointClaudeMessages, EndpointOpenAIChat, EndpointOpenAIResponses, EndpointGemini}
)

// IsEndpoint reports whether endpoint names an API that rules can apply to.
func IsEndpoint(endpoint string) bool {
	return slices.Contains(endpoints, endpoint)
}

type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Preset marks a rule shipped with the gateway; Presets holds its default.
	Preset    bool     `json:"preset"`
	Enabled   bool     `json:"enabled"`
	Endpoints []string `json:"endpoints"`
	// ModelRegex matches the model the client asked for; empty matches all.
	ModelRegex string `json:"model_regex"`
	// OnlyPassThrough checks the rule only when the channel relays the body
	// unchanged, for fields a rebuilt body drops or fills in.
	OnlyPassThrough bool        `json:"only_pass_through"`
	Match           string      `json:"match"`
	Conditions      []Condition `json:"conditions"`
	Response        Response    `json:"response"`
}

type Condition struct {
	Source string `json:"source"`
	// Path is a JSON field path for json conditions and a header name for
	// header conditions. Paths use gjson keys plus "#" (every array item),
	// "-1" (the last item) and a final "@keys" (the object's field names).
	Path  string `json:"path"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// Response is the error a matched request gets. Message may use {{rule}},
// {{model}}, {{path}}, {{field}} and {{value}}.
type Response struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Input is one client request as the rules see it.
type Input struct {
	Endpoint    string
	Model       string
	Path        string
	PassThrough bool
	Body        []byte
	Header      http.Header
}

// Match is the first rule a request matched.
type Match struct {
	Index   int
	Rule    Rule
	Field   string
	Value   string
	Message string
}

// APIError is the error the gateway answers the matched request with. It is
// the caller's error, so it is not retried, and its message is shown as the
// administrator wrote it.
func (m *Match) APIError() *types.NewAPIError {
	return types.NewErrorWithStatusCode(errors.New(m.Message), types.ErrorCode(m.Rule.Response.Code), m.Rule.Response.Status,
		types.ErrOptionWithSkipRetry(), types.ErrOptionWithPlainMessage())
}

// Trace results explain, per rule, why it did or did not match.
const (
	TraceDisabled    = "disabled"
	TraceEndpoint    = "endpoint"
	TraceModel       = "model"
	TracePassThrough = "pass_through"
	TraceConditions  = "conditions"
	TraceMatched     = "matched"
	TraceNotChecked  = "not_checked"
)

type Trace struct {
	Position int    `json:"position"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Result   string `json:"result"`
}

// Set is a validated, compiled rule list.
type Set struct {
	rules []compiledRule
}

type compiledRule struct {
	rule       Rule
	model      *regexp.Regexp
	conditions []compiledCondition
	needsJSON  bool
}

type compiledCondition struct {
	Condition
	re   *regexp.Regexp
	path []string
}

// location is where a condition matched. A concrete location names one field
// and the value found there; otherwise field is the condition's path.
type location struct {
	field    string
	value    string
	concrete bool
}

// Parse validates and compiles a JSON array of rules; an empty string is an
// empty rule list. Errors name the rule and condition that failed.
func Parse(raw string) (*Set, error) {
	set := &Set{}
	if strings.TrimSpace(raw) == "" {
		return set, nil
	}
	var rules []Rule
	if err := common.UnmarshalJsonStr(raw, &rules); err != nil {
		return nil, fmt.Errorf("request rules must be a JSON array of rules: %w", err)
	}
	if len(rules) > maxRules {
		return nil, fmt.Errorf("at most %d request rules are allowed", maxRules)
	}
	seen := make(map[string]bool, len(rules))
	for i, rule := range rules {
		compiled, err := compileRule(rule)
		if err == nil && seen[rule.ID] {
			err = fmt.Errorf("duplicate id %q", rule.ID)
		}
		if err != nil {
			label := rule.Name
			if label == "" {
				label = rule.ID
			}
			return nil, fmt.Errorf("rule %d (%s): %w", i+1, label, err)
		}
		seen[rule.ID] = true
		set.rules = append(set.rules, compiled)
	}
	return set, nil
}

func compileRule(rule Rule) (compiledRule, error) {
	compiled := compiledRule{rule: rule}
	if !idPattern.MatchString(rule.ID) {
		return compiled, errors.New("id must be 1-64 letters, digits, dashes or underscores")
	}
	if name := strings.TrimSpace(rule.Name); name == "" || utf8.RuneCountInString(name) > maxNameRunes {
		return compiled, fmt.Errorf("name must be 1-%d characters", maxNameRunes)
	}
	if utf8.RuneCountInString(rule.Description) > maxDescRunes {
		return compiled, fmt.Errorf("description must be at most %d characters", maxDescRunes)
	}
	if len(rule.Endpoints) == 0 {
		return compiled, errors.New("select at least one endpoint")
	}
	for i, endpoint := range rule.Endpoints {
		if !IsEndpoint(endpoint) {
			return compiled, fmt.Errorf("unknown endpoint %q", endpoint)
		}
		if slices.Contains(rule.Endpoints[:i], endpoint) {
			return compiled, fmt.Errorf("endpoint %q is listed twice", endpoint)
		}
	}
	if rule.ModelRegex != "" {
		re, err := compilePattern(rule.ModelRegex)
		if err != nil {
			return compiled, fmt.Errorf("model regex: %w", err)
		}
		compiled.model = re
	}
	switch rule.Match {
	case "", MatchAll, MatchAny:
	default:
		return compiled, fmt.Errorf("match must be %q or %q", MatchAll, MatchAny)
	}
	if len(rule.Conditions) == 0 || len(rule.Conditions) > maxConditions {
		return compiled, fmt.Errorf("a rule needs 1-%d conditions", maxConditions)
	}
	for i, condition := range rule.Conditions {
		cc, err := compileCondition(condition)
		if err != nil {
			return compiled, fmt.Errorf("condition %d: %w", i+1, err)
		}
		compiled.needsJSON = compiled.needsJSON || cc.Source == SourceJSON
		compiled.conditions = append(compiled.conditions, cc)
	}
	response := rule.Response
	if response.Status < 400 || response.Status > 499 {
		return compiled, errors.New("status code must be between 400 and 499")
	}
	if !codePattern.MatchString(response.Code) {
		return compiled, errors.New("error code must be 1-64 letters, digits, dots, dashes or underscores")
	}
	if message := strings.TrimSpace(response.Message); message == "" || utf8.RuneCountInString(message) > maxMessageRunes {
		return compiled, fmt.Errorf("error message must be 1-%d characters", maxMessageRunes)
	}
	return compiled, nil
}

func compileCondition(condition Condition) (compiledCondition, error) {
	cc := compiledCondition{Condition: condition}
	switch condition.Source {
	case SourceJSON, SourceHeader:
		if condition.Path == "" || len(condition.Path) > maxPathLength {
			return cc, fmt.Errorf("path must be 1-%d characters", maxPathLength)
		}
	case SourceBody:
		if condition.Op == OpExists || condition.Op == OpNotExists {
			return cc, errors.New("the request body can only be matched with a regex")
		}
	default:
		return cc, fmt.Errorf("unknown source %q", condition.Source)
	}
	if condition.Source == SourceJSON {
		cc.path = splitPath(condition.Path)
		for i, segment := range cc.path {
			if segment == "" {
				return cc, fmt.Errorf("path %q has an empty segment", condition.Path)
			}
			if segment == "@keys" && i != len(cc.path)-1 {
				return cc, errors.New("@keys must be the last segment of a path")
			}
		}
	}
	switch condition.Op {
	case OpRegex, OpNotRegex:
		if condition.Value == "" {
			return cc, errors.New("regex is required")
		}
		re, err := compilePattern(condition.Value)
		if err != nil {
			return cc, err
		}
		cc.re = re
	case OpExists, OpNotExists:
	default:
		return cc, fmt.Errorf("unknown operator %q", condition.Op)
	}
	return cc, nil
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > maxPatternLength {
		return nil, fmt.Errorf("regex must be at most %d characters", maxPatternLength)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	return re, nil
}

// splitPath splits a field path at dots; "\." keeps a literal dot in a key.
func splitPath(path string) []string {
	var segments []string
	var segment strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+1 < len(path) {
			segment.WriteByte(path[i+1])
			i++
			continue
		}
		if path[i] == '.' {
			segments = append(segments, segment.String())
			segment.Reset()
			continue
		}
		segment.WriteByte(path[i])
	}
	return append(segments, segment.String())
}

func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	return len(s.rules)
}

// AppliesTo reports whether an enabled rule checks requests to endpoint, so
// callers can skip reading the request body when none does.
func (s *Set) AppliesTo(endpoint string) bool {
	if s == nil {
		return false
	}
	for i := range s.rules {
		if s.rules[i].rule.Enabled && slices.Contains(s.rules[i].rule.Endpoints, endpoint) {
			return true
		}
	}
	return false
}

// Match returns the first enabled rule the request matches, or nil.
func (s *Set) Match(in Input) *Match {
	match, _ := s.evaluate(in, false)
	return match
}

// Explain is Match plus the reason each rule did or did not match.
func (s *Set) Explain(in Input) (*Match, []Trace) {
	return s.evaluate(in, true)
}

func (s *Set) evaluate(in Input, explain bool) (*Match, []Trace) {
	if s == nil {
		return nil, nil
	}
	var traces []Trace
	var match *Match
	var root gjson.Result
	parsed := false
	for i := range s.rules {
		cr := &s.rules[i]
		result := TraceNotChecked
		switch {
		case match != nil:
		case !cr.rule.Enabled:
			result = TraceDisabled
		case !slices.Contains(cr.rule.Endpoints, in.Endpoint):
			result = TraceEndpoint
		case cr.model != nil && !cr.model.MatchString(in.Model):
			result = TraceModel
		case cr.rule.OnlyPassThrough && !in.PassThrough:
			result = TracePassThrough
		default:
			if cr.needsJSON && !parsed {
				// The body is not modified while rules run, so read it in place.
				root = gjson.Parse(unsafe.String(unsafe.SliceData(in.Body), len(in.Body)))
				parsed = true
			}
			loc, ok := cr.matchConditions(root, in)
			if !ok {
				result = TraceConditions
				break
			}
			result = TraceMatched
			value := loc.value
			if utf8.RuneCountInString(value) > maxValueRunes {
				value = string([]rune(value)[:maxValueRunes]) + "…"
			}
			match = &Match{Index: i, Rule: cr.rule, Field: loc.field, Value: value}
			match.Message = strings.NewReplacer(
				"{{rule}}", cr.rule.Name,
				"{{model}}", in.Model,
				"{{path}}", in.Path,
				"{{field}}", match.Field,
				"{{value}}", match.Value,
			).Replace(cr.rule.Response.Message)
		}
		if !explain {
			if match != nil {
				return match, nil
			}
			continue
		}
		traces = append(traces, Trace{Position: i + 1, ID: cr.rule.ID, Name: cr.rule.Name, Result: result})
	}
	return match, traces
}

// matchConditions applies the rule's match mode. The reported location is
// the first condition that pointed at a concrete field, when there is one.
func (cr *compiledRule) matchConditions(root gjson.Result, in Input) (location, bool) {
	anyMode := cr.rule.Match == MatchAny
	var picked location
	for i := range cr.conditions {
		loc, hit := cr.conditions[i].check(root, in)
		if anyMode && hit {
			return loc, true
		}
		if !anyMode && !hit {
			return location{}, false
		}
		if hit && (picked.field == "" || loc.concrete && !picked.concrete) {
			picked = loc
		}
	}
	return picked, !anyMode
}

func (cc *compiledCondition) check(root gjson.Result, in Input) (location, bool) {
	if cc.Source == SourceBody {
		body := in.Body[:min(len(in.Body), maxBodyScanBytes)]
		text := unsafe.String(unsafe.SliceData(body), len(body))
		span := cc.re.FindStringIndex(text)
		if cc.Op == OpRegex {
			if span == nil {
				return location{}, false
			}
			return location{field: "body", value: text[span[0]:span[1]], concrete: true}, true
		}
		return location{field: "body"}, span == nil
	}
	var found []location
	if cc.Source == SourceHeader {
		for _, value := range in.Header.Values(cc.Path) {
			found = append(found, location{field: cc.Path, value: value, concrete: true})
		}
	} else {
		collect(root, cc.path, "", &found)
	}
	switch cc.Op {
	case OpExists:
		if len(found) == 0 {
			return location{}, false
		}
		return found[0], true
	case OpNotExists:
		return location{field: cc.Path}, len(found) == 0
	}
	for _, loc := range found {
		if cc.re.MatchString(loc.value) {
			if cc.Op == OpRegex {
				return loc, true
			}
			return location{}, false
		}
	}
	return location{field: cc.Path}, cc.Op == OpNotRegex
}

// collect appends every value the path reaches. Fields are reported with
// array indexes in brackets, for example messages[1].content[0].type.
func collect(res gjson.Result, segments []string, field string, out *[]location) {
	if !res.Exists() {
		return
	}
	if len(segments) == 0 {
		value := res.Raw
		if res.Type == gjson.String {
			value = res.Str
		}
		*out = append(*out, location{field: field, value: value, concrete: true})
		return
	}
	segment, rest := segments[0], segments[1:]
	switch {
	case segment == "#":
		if res.IsArray() {
			for i, item := range res.Array() {
				collect(item, rest, indexField(field, i), out)
			}
		}
	case segment == "-1":
		if res.IsArray() {
			if items := res.Array(); len(items) > 0 {
				collect(items[len(items)-1], rest, indexField(field, len(items)-1), out)
			}
		}
	case segment == "@keys":
		if res.IsObject() {
			for _, key := range res.Get("@keys").Array() {
				*out = append(*out, location{field: keyField(field, key.Str), value: key.Str, concrete: true})
			}
		}
	case res.IsArray():
		if i, err := strconv.Atoi(segment); err == nil && i >= 0 {
			collect(res.Get(segment), rest, indexField(field, i), out)
		}
	default:
		collect(res.Get(gjson.Escape(segment)), rest, keyField(field, segment), out)
	}
}

func indexField(field string, index int) string {
	return field + "[" + strconv.Itoa(index) + "]"
}

func keyField(field string, key string) string {
	if field == "" {
		return key
	}
	return field + "." + key
}

type cacheEntry struct {
	raw string
	set *Set
}

var cached atomic.Pointer[cacheEntry]

// Cached returns the compiled rules for raw and reuses them while raw stays
// the same. Saving validates rules first, so a list that fails to compile
// here was stored by other means; it matches nothing rather than failing
// every request.
func Cached(raw string) *Set {
	if entry := cached.Load(); entry != nil && entry.raw == raw {
		return entry.set
	}
	set, err := Parse(raw)
	if err != nil {
		common.SysError("invalid request rules ignored: " + err.Error())
		set = &Set{}
	}
	cached.Store(&cacheEntry{raw: raw, set: set})
	return set
}
