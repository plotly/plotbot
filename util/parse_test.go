package util

import (
	"strconv"
	"testing"
)

type ParseDaysTest struct {
	Test         string
	ExpectedDays int
}

var tests = []ParseDaysTest{
	ParseDaysTest{"plot, give me a report for the past 5 days", 5},
	ParseDaysTest{"plot, give me a report for the past day", 1},
	ParseDaysTest{"plot, give me a report for the last week", 7},
	ParseDaysTest{"plot, give me a report for the past 2 week", 14},
	ParseDaysTest{"plot, give me a report for today", 0},
	ParseDaysTest{"first as tragedy, second as farce", 0},
	ParseDaysTest{"plot, give me a report for this week", 7},
}

func TestParseDays(t *testing.T) {
	for _, test := range tests {
		if days := ParseDays(test.Test); days != test.ExpectedDays {
			message := "expected '" + test.Test + "' "
			message += "to yield " + strconv.Itoa(test.ExpectedDays) + " days "
			message += "and got " + strconv.Itoa(days)
			t.Error(message)
		}
	}
}
