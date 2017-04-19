package util

import (
	"regexp"
	"strconv"
)

var DEFAULT_DAYS = 7

func GetDaysFromQuery(text string) int {

	re := regexp.MustCompile(".*(?:last|past|this) (\\d+)?\\s?(day|week).*")
	hits := re.FindStringSubmatch(text)

	days := DEFAULT_DAYS
	var weeks int
	var err error

	if len(hits) == 3 {
		howmany := hits[1]
		dayOrWeek := hits[2]

		if dayOrWeek == "day" {
			if howmany == "" {
				days = 1
			} else {
				days, err = strconv.Atoi(howmany)
				if err != nil {
					days = 1
				}
			}
		} else {
			if howmany == "" {
				days = 7
			} else {
				weeks, err = strconv.Atoi(howmany)
				if err != nil {
					days = 7
				} else {
					days = 7 * weeks
				}
			}
		}
	}

	return days
}
