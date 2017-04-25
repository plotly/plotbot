package util

import (
	"fmt"
	"strings"
)

type Searchable []string

func (ps Searchable) Contains(s string) bool {
	for _, p := range ps {
		if strings.Contains(p, s) {
			return true
		}
	}

	return false
}

func (ps Searchable) ContainsAll(ss ...string) bool {
	for _, s := range ss {
		if !ps.Contains(s) {
			return false
		}
	}

	return true
}

func (ps Searchable) ContainsAny(ss ...string) bool {
	for _, s := range ss {
		if ps.Contains(s) {
			return true
		}
	}

	return false
}

func (ps Searchable) Includes(s string) bool {
	for _, p := range ps {
		if p == s {
			return true
		}
	}

	return false
}

func (ps Searchable) IncludesAll(ss ...string) bool {
	for _, s := range ss {
		if !ps.Includes(s) {
			return false
		}
	}

	return true
}

func (ps Searchable) IncludesAny(ss ...string) bool {
	for _, s := range ss {
		if ps.Includes(s) {
			return true
		}
	}

	return false
}

func (ps Searchable) String() string {
	return fmt.Sprintf("[%s]", strings.Join(ps, ", "))
}
