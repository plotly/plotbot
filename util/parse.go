package util

import (
	"regexp"
	"strconv"
)

func ParseDays(text string) int {
	re := regexp.MustCompile(".*(?:last|past|this) (\\d+)?\\s?(day|week).*")
	matches := re.FindStringSubmatch(text)
	days := 0

	if len(matches) == 3 {
		countStr := matches[1]
		dayOrWeek := matches[2]

		if dayOrWeek == "day" {
			days = 1
			if countStr != "" {
				if count, err := strconv.Atoi(countStr); err == nil {
					days = count
				}
			}
		} else {
			days = 7
			if countStr != "" {
				if count, err := strconv.Atoi(countStr); err == nil {
					days = 7 * count
				}
			}
		}
	}

	return days
}
