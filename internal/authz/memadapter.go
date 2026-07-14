package authz

import (
	"strings"
	"sync"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

// memAdapter is a minimal in-memory persist.Adapter for casbin. The policy
// lines are the single source of truth; LoadPolicy parses them into the model
// and SavePolicy re-serialises the model back. Unlike casbin's string-adapter,
// AddPolicy / RemovePolicy mutate the live policy so the enforcer can be
// updated at runtime (required for Reload after role/permission changes).
type memAdapter struct {
	mu    sync.Mutex
	lines []string
}

func newMemAdapter() *memAdapter { return &memAdapter{} }

func (a *memAdapter) LoadPolicy(m model.Model) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, line := range a.lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		persist.LoadPolicyLine(line, m)
	}
	return nil
}

func (a *memAdapter) SavePolicy(m model.Model) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var b strings.Builder
	for ptype, ast := range m["p"] {
		for _, rule := range ast.Policy {
			b.WriteString(ptype)
			b.WriteString(", ")
			b.WriteString(strings.Join(rule, ", "))
			b.WriteString("\n")
		}
	}
	for ptype, ast := range m["g"] {
		for _, rule := range ast.Policy {
			b.WriteString(ptype)
			b.WriteString(", ")
			b.WriteString(strings.Join(rule, ", "))
			b.WriteString("\n")
		}
	}
	trimmed := strings.TrimRight(b.String(), "\n")
	if trimmed == "" {
		a.lines = nil
		return nil
	}
	a.lines = strings.Split(trimmed, "\n")
	return nil
}

func (a *memAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lines = append(a.lines, ptype+", "+strings.Join(rule, ", "))
	return nil
}

func (a *memAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	target := ptype + ", " + strings.Join(rule, ", ")
	out := a.lines[:0]
	for _, l := range a.lines {
		if l != target {
			out = append(out, l)
		}
	}
	a.lines = out
	return nil
}

func (a *memAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.lines))
	for _, l := range a.lines {
		if !matchFiltered(l, ptype, fieldIndex, fieldValues) {
			out = append(out, l)
		}
	}
	a.lines = out
	return nil
}

// AddPolicies / RemovePolicies implement persist.BatchAdapter (casbin performs
// an unchecked type assertion to BatchAdapter, so it must be satisfied).
func (a *memAdapter) AddPolicies(sec string, ptype string, rules [][]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, rule := range rules {
		a.lines = append(a.lines, ptype+", "+strings.Join(rule, ", "))
	}
	return nil
}

func (a *memAdapter) RemovePolicies(sec string, ptype string, rules [][]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	targets := make(map[string]bool, len(rules))
	for _, rule := range rules {
		targets[ptype+", "+strings.Join(rule, ", ")] = true
	}
	out := a.lines[:0]
	for _, l := range a.lines {
		if !targets[l] {
			out = append(out, l)
		}
	}
	a.lines = out
	return nil
}

func matchFiltered(line, ptype string, fieldIndex int, fieldValues []string) bool {
	parts := strings.Split(line, ", ")
	if len(parts) == 0 || parts[0] != ptype {
		return false
	}
	rule := parts[1:]
	if fieldIndex >= len(rule) {
		return false
	}
	for i, v := range fieldValues {
		if v == "" {
			continue
		}
		if fieldIndex+i >= len(rule) || rule[fieldIndex+i] != v {
			return false
		}
	}
	return true
}
