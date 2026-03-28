package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// ErrRuleNotFound is returned when attempting to remove a rule that does not exist.
var ErrRuleNotFound = errors.New("rule not found")

// Rule represents a single correction rule.
type Rule struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Scope  string `json:"scope,omitempty"`  // e.g., "git", "docker", "npm"
	Enable bool   `json:"enable,omitempty"` // default true
}

// RuleSet represents a collection of rules.
type RuleSet struct {
	Name   string `json:"name"`
	Enable bool   `json:"enable"`
	Rules  []Rule `json:"rules"`
}

// Rules manages correction rules.
type Rules struct {
	mu        sync.RWMutex
	builtin   map[string]Rule    // from -> Rule, compiled into binary
	user      map[string]Rule    // from -> Rule, loaded from user config
	ruleSets  map[string]RuleSet // scope -> RuleSet
	configDir string
}

// NewRules creates a new Rules instance.
func NewRules(configDir string) *Rules {
	r := &Rules{
		builtin:   make(map[string]Rule),
		user:      make(map[string]Rule),
		ruleSets:  make(map[string]RuleSet),
		configDir: configDir,
	}
	r.initBuiltinRules()
	r.loadUserRules()
	return r
}

// Match finds a matching rule for the given command.
// Returns the rule and true if found, empty rule and false otherwise.
func (r *Rules) Match(cmd string) (Rule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// User rules have priority
	if rule, ok := r.user[cmd]; ok && rule.Enable {
		return rule, true
	}

	// Then builtin rules
	if rule, ok := r.builtin[cmd]; ok && rule.Enable {
		return rule, true
	}

	return Rule{}, false
}

// MatchUser finds a matching user rule for the given command.
func (r *Rules) MatchUser(cmd string) (Rule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rule, ok := r.user[cmd]
	if !ok || !rule.Enable {
		return Rule{}, false
	}

	return rule, true
}

// MatchBuiltin finds a matching builtin rule for the given command.
func (r *Rules) MatchBuiltin(cmd string) (Rule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rule, ok := r.builtin[cmd]
	if !ok || !rule.Enable {
		return Rule{}, false
	}

	return rule, true
}

// AddUserRule adds a new user rule.
func (r *Rules) AddUserRule(rule Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rule.Enable = true
	r.user[rule.From] = rule
	return r.saveUserRules()
}

// RemoveUserRule removes a user rule.
func (r *Rules) RemoveUserRule(from string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.user[from]; !exists {
		return ErrRuleNotFound
	}
	delete(r.user, from)
	return r.saveUserRules()
}

// ListRules returns all rules (builtin + user).
func (r *Rules) ListRules() []Rule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]Rule, 0, len(r.builtin)+len(r.user))

	// Add builtin rules
	for _, rule := range r.builtin {
		rules = append(rules, rule)
	}

	// Add user rules
	for _, rule := range r.user {
		rules = append(rules, rule)
	}

	return rules
}

// TargetPriority returns the explicit preference score for a correction target.
func (r *Rules) TargetPriority(cmd string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	score := 0
	for _, rule := range r.user {
		if rule.Enable && rule.To == cmd {
			score += 200
		}
	}

	for _, rule := range r.builtin {
		if rule.Enable && rule.To == cmd {
			score += 100
		}
	}

	return score
}

// EnableRuleSet enables or disables a rule set by scope.
func (r *Rules) EnableRuleSet(scope string, enable bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rs, ok := r.ruleSets[scope]; ok {
		rs.Enable = enable
		r.ruleSets[scope] = rs
	}

	// Update all rules in this scope
	for from, rule := range r.builtin {
		if rule.Scope == scope {
			rule.Enable = enable
			r.builtin[from] = rule
		}
	}

	return r.saveUserRules()
}

// GetRuleSets returns all rule sets.
func (r *Rules) GetRuleSets() []RuleSet {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sets := make([]RuleSet, 0, len(r.ruleSets))
	for _, rs := range r.ruleSets {
		sets = append(sets, rs)
	}
	return sets
}

func (r *Rules) initBuiltinRules() {
	builtinRules := []Rule{
		// Git rules
		{From: "gut", To: "git", Scope: "git"},
		{From: "gti", To: "git", Scope: "git"},
		{From: "got", To: "git", Scope: "git"},
		{From: "gi", To: "git", Scope: "git"},
		{From: "igt", To: "git", Scope: "git"},
		{From: "grt", To: "git", Scope: "git"},

		// Docker rules
		{From: "dcoker", To: "docker", Scope: "docker"},
		{From: "dokcer", To: "docker", Scope: "docker"},
		{From: "docke", To: "docker", Scope: "docker"},
		{From: "dockr", To: "docker", Scope: "docker"},
		{From: "doker", To: "docker", Scope: "docker"},

		// npm rules
		{From: "npn", To: "npm", Scope: "npm"},
		{From: "nmp", To: "npm", Scope: "npm"},
		{From: "npmi", To: "npm", Scope: "npm"},

		// yarn rules
		{From: "yran", To: "yarn", Scope: "yarn"},
		{From: "yanr", To: "yarn", Scope: "yarn"},
		{From: "yam", To: "yarn", Scope: "yarn"},

		// kubectl rules
		{From: "kubctl", To: "kubectl", Scope: "kubectl"},
		{From: "kubetcl", To: "kubectl", Scope: "kubectl"},
		{From: "kubecdl", To: "kubectl", Scope: "kubectl"},
		{From: "kuebctl", To: "kubectl", Scope: "kubectl"},

		// cargo rules
		{From: "crago", To: "cargo", Scope: "cargo"},
		{From: "cago", To: "cargo", Scope: "cargo"},

		// brew rules
		{From: "bre", To: "brew", Scope: "brew"},
		{From: "brwe", To: "brew", Scope: "brew"},

		// helm rules
		{From: "helmm", To: "helm", Scope: "helm"},
		{From: "hlem", To: "helm", Scope: "helm"},

		// terraform rules
		{From: "terrafrom", To: "terraform", Scope: "terraform"},

		// python rules
		{From: "pyhton", To: "python", Scope: "python"},
		{From: "pyton", To: "python", Scope: "python"},
		{From: "pythn", To: "python", Scope: "python"},
		{From: "pyhton3", To: "python3", Scope: "python"},
		{From: "pythno3", To: "python3", Scope: "python"},
		{From: "python33", To: "python3", Scope: "python"},

		// pip rules
		{From: "pi", To: "pip", Scope: "pip"},
		{From: "pipi", To: "pip", Scope: "pip"},

		// go rules
		{From: "og", To: "go", Scope: "go"},

		// ls rules
		{From: "lls", To: "ls", Scope: "system"},
		{From: "sl", To: "ls", Scope: "system"},
		{From: "l", To: "ls", Scope: "system"},

		// cd/pwd rules
		{From: "dc", To: "cd", Scope: "system"},
		{From: "xd", To: "cd", Scope: "system"},
		{From: "pdw", To: "pwd", Scope: "system"},

		// cat rules
		{From: "cta", To: "cat", Scope: "system"},
		{From: "act", To: "cat", Scope: "system"},

		// grep rules
		{From: "gerp", To: "grep", Scope: "system"},
		{From: "grpe", To: "grep", Scope: "system"},

		// echo rules
		{From: "ehco", To: "echo", Scope: "system"},
		{From: "ehoc", To: "echo", Scope: "system"},

		// shell builtin and utility rules
		{From: "sourc", To: "source", Scope: "system"},
		{From: "printff", To: "printf", Scope: "system"},
		{From: "exportt", To: "export", Scope: "system"},
		{From: "unsett", To: "unset", Scope: "system"},
		{From: "historyy", To: "history", Scope: "system"},
		{From: "typ", To: "type", Scope: "system"},
		{From: "hasj", To: "hash", Scope: "system"},
		{From: "helpp", To: "help", Scope: "system"},
		{From: "testt", To: "test", Scope: "system"},
		{From: "truee", To: "true", Scope: "system"},
		{From: "falsee", To: "false", Scope: "system"},
		{From: "evall", To: "eval", Scope: "system"},
		{From: "execc", To: "exec", Scope: "system"},

		// sudo rules
		{From: "sduo", To: "sudo", Scope: "system"},
		{From: "sodo", To: "sudo", Scope: "system"},
		{From: "suod", To: "sudo", Scope: "system"},

		// make rules
		{From: "maek", To: "make", Scope: "system"},
		{From: "mkae", To: "make", Scope: "system"},

		// curl rules
		{From: "crul", To: "curl", Scope: "system"},
		{From: "culr", To: "curl", Scope: "system"},

		// tar rules
		{From: "tra", To: "tar", Scope: "system"},
		{From: "atr", To: "tar", Scope: "system"},

		// common file and process rules
		{From: "taill", To: "tail", Scope: "system"},
		{From: "headd", To: "head", Scope: "system"},
		{From: "xagrs", To: "xargs", Scope: "system"},
		{From: "xargss", To: "xargs", Scope: "system"},
		{From: "srot", To: "sort", Scope: "system"},
		{From: "unqi", To: "uniq", Scope: "system"},
		{From: "ctu", To: "cut", Scope: "system"},
		{From: "teee", To: "tee", Scope: "system"},
		{From: "wcc", To: "wc", Scope: "system"},
		{From: "whcih", To: "which", Scope: "system"},
		{From: "lesss", To: "less", Scope: "system"},
		{From: "mkdi", To: "mkdir", Scope: "system"},
		{From: "rmm", To: "rm", Scope: "system"},
		{From: "cpx", To: "cp", Scope: "system"},
		{From: "mvv", To: "mv", Scope: "system"},
		{From: "touc", To: "touch", Scope: "system"},
		{From: "fin", To: "find", Scope: "system"},
		{From: "sedd", To: "sed", Scope: "system"},
		{From: "awkk", To: "awk", Scope: "system"},

		// chmod/chown rules
		{From: "chmdo", To: "chmod", Scope: "system"},
		{From: "chwon", To: "chown", Scope: "system"},
		{From: "chwno", To: "chown", Scope: "system"},
		{From: "chownn", To: "chown", Scope: "system"},

		// ssh/scp rules
		{From: "shs", To: "ssh", Scope: "system"},
		{From: "spc", To: "scp", Scope: "system"},
		{From: "sshh", To: "ssh", Scope: "system"},
		{From: "scpp", To: "scp", Scope: "system"},

		// wget/gzip rules
		{From: "wegt", To: "wget", Scope: "system"},
		{From: "wgte", To: "wget", Scope: "system"},
		{From: "wgett", To: "wget", Scope: "system"},
		{From: "gizp", To: "gzip", Scope: "system"},
		{From: "gzpi", To: "gzip", Scope: "system"},
		{From: "gzipp", To: "gzip", Scope: "system"},
		{From: "rsycn", To: "rsync", Scope: "system"},
		{From: "unzpi", To: "unzip", Scope: "system"},

		// runtime and process rules
		{From: "ndoe", To: "node", Scope: "system"},
		{From: "nodee", To: "node", Scope: "system"},
		{From: "klli", To: "kill", Scope: "system"},
		{From: "killl", To: "kill", Scope: "system"},
		{From: "zpi", To: "zip", Scope: "system"},
		{From: "zipp", To: "zip", Scope: "system"},
		{From: "suu", To: "su", Scope: "system"},
		{From: "pss", To: "ps", Scope: "system"},
		{From: "lnn", To: "ln", Scope: "system"},
		{From: "duu", To: "du", Scope: "system"},
		{From: "dff", To: "df", Scope: "system"},
		{From: "daet", To: "date", Scope: "system"},
		{From: "opne", To: "open", Scope: "system"},
		{From: "claer", To: "clear", Scope: "system"},
		{From: "mna", To: "man", Scope: "system"},
		{From: "whomai", To: "whoami", Scope: "system"},
		{From: "unmae", To: "uname", Scope: "system"},
		{From: "basenmae", To: "basename", Scope: "system"},
		{From: "dirnmae", To: "dirname", Scope: "system"},
		{From: "fiel", To: "file", Scope: "system"},
		{From: "stta", To: "stat", Scope: "system"},

		// Java
		{From: "jvav", To: "java", Scope: "java"},
		{From: "javv", To: "java", Scope: "java"},
		{From: "jaav", To: "java", Scope: "java"},
	}

	for _, rule := range builtinRules {
		rule.Enable = true
		r.builtin[rule.From] = rule
	}

	// Initialize rule sets
	r.ruleSets = map[string]RuleSet{
		"git":       {Name: "git", Enable: true},
		"docker":    {Name: "docker", Enable: true},
		"npm":       {Name: "npm", Enable: true},
		"yarn":      {Name: "yarn", Enable: true},
		"kubectl":   {Name: "kubectl", Enable: true},
		"cargo":     {Name: "cargo", Enable: true},
		"brew":      {Name: "brew", Enable: true},
		"helm":      {Name: "helm", Enable: true},
		"terraform": {Name: "terraform", Enable: true},
		"python":    {Name: "python", Enable: true},
		"pip":       {Name: "pip", Enable: true},
		"go":        {Name: "go", Enable: true},
		"java":      {Name: "java", Enable: true},
		"system":    {Name: "system", Enable: true},
	}
}

func (r *Rules) loadUserRules() {
	rulesFile := filepath.Join(r.configDir, "rules.json")
	data, err := os.ReadFile(rulesFile)
	if err != nil {
		return // No user rules file yet
	}

	var userRules []Rule
	if err := json.Unmarshal(data, &userRules); err != nil {
		return // Invalid JSON, ignore
	}

	for _, rule := range userRules {
		rule.Enable = true
		r.user[rule.From] = rule
	}
}

func (r *Rules) saveUserRules() error {
	if r.configDir == "" {
		return nil
	}

	if err := os.MkdirAll(r.configDir, 0755); err != nil {
		return err
	}

	rules := make([]Rule, 0, len(r.user))
	for _, rule := range r.user {
		rules = append(rules, rule)
	}

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	rulesFile := filepath.Join(r.configDir, "rules.json")
	return os.WriteFile(rulesFile, data, 0600)
}
