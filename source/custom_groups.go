package main

import "strings"

// customGroups stores ordered group names and stable entry-to-group assignments.
type customGroups struct {
	Names       []string          `json:"names"`
	Assignments map[string]string `json:"assignments"`
}

func (g *customGroups) normalize() {
	seen := make(map[string]struct{}, len(g.Names))
	names := g.Names[:0]
	for _, name := range g.Names {
		name = strings.TrimSpace(name)
		key := strings.ToLower(name)
		if name == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	g.Names = names
	if g.Assignments == nil {
		g.Assignments = make(map[string]string)
	}
	for entry, group := range g.Assignments {
		canonical := ""
		for _, name := range g.Names {
			if strings.EqualFold(name, group) {
				canonical = name
				break
			}
		}
		if canonical == "" {
			delete(g.Assignments, entry)
		} else {
			g.Assignments[entry] = canonical
		}
	}
}

func (g *customGroups) add(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	g.normalize()
	for _, existing := range g.Names {
		if strings.EqualFold(existing, name) {
			return existing
		}
	}
	g.Names = append(g.Names, name)
	return name
}

func (g *customGroups) assign(entry, group string) {
	g.normalize()
	if group == "" {
		delete(g.Assignments, entry)
		return
	}
	for _, name := range g.Names {
		if strings.EqualFold(name, group) {
			g.Assignments[entry] = name
			return
		}
	}
}

func (g *customGroups) group(entry string) string {
	g.normalize()
	return g.Assignments[entry]
}

func (g *customGroups) remove(name string) {
	g.normalize()
	for i, existing := range g.Names {
		if strings.EqualFold(existing, name) {
			g.Names = append(g.Names[:i], g.Names[i+1:]...)
			break
		}
	}
	for entry, assigned := range g.Assignments {
		if strings.EqualFold(assigned, name) {
			delete(g.Assignments, entry)
		}
	}
}

func playerCustomGroupKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
