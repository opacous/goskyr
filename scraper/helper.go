package scraper

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"regexp"
	"strings"
)

func GetTextString(t string, s *goquery.Selection) (fieldString []string, err error) {
	fieldString = s.Find(t).Map(
		func(_ int, s *goquery.Selection) string {
			return strings.TrimSpace(s.Text())
		},
	)
	return fieldString, err
}

func ExtractStringRegex(rc *RegexConfig, s string) (string, error) {
	extractedString := s
	if rc.Exp != "" {
		regex, err := regexp.Compile(rc.Exp)
		if err != nil {
			return "", err
		}
		matchingStrings := regex.FindAllString(s, -1)
		if len(matchingStrings) == 0 {
			msg := fmt.Sprintf("no matching strings found for regex: %s", rc.Exp)
			return "", errors.New(msg)
		}
		if rc.Index == -1 {
			extractedString = matchingStrings[len(matchingStrings)-1]
		} else {
			if rc.Index >= len(matchingStrings) {
				msg := fmt.Sprintf("regex index out of bounds. regex '%s' gave only %d matches", rc.Exp, len(matchingStrings))
				return "", errors.New(msg)
			}
			extractedString = matchingStrings[rc.Index]
		}
	}
	return extractedString, nil
}
