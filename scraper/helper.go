package scraper

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

func GetTextString(t *ElementLocation, s *goquery.Selection) (string, error) {
	var fieldString string
	var err error
	fieldSelection := s.Find(t.Selector)
	if len(fieldSelection.Nodes) > t.NodeIndex {
		if t.Attr == "" {
			if t.EntireSubtree {
				// copied from https://github.com/PuerkitoBio/goquery/blob/v1.8.0/property.go#L62
				var buf bytes.Buffer
				var f func(*html.Node)
				f = func(n *html.Node) {
					if n.Type == html.TextNode {
						// Keep newlines and spaces, like jQuery
						buf.WriteString(n.Data)
					}
					if n.FirstChild != nil {
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							f(c)
						}
					}
				}
				f(fieldSelection.Get(t.NodeIndex))
				fieldString = buf.String()
			} else {
				fieldNode := fieldSelection.Get(t.NodeIndex).FirstChild
				currentChildIndex := 0
				for fieldNode != nil {
					// for the case where we want to find the correct string
					// by regex (checking all the children and taking the first one that matches the regex)
					// the ChildIndex has to be set to -1 to
					// distinguish from the default case 0. So when we explicitly set ChildIndex to -1 it means
					// check _all_ of the children.
					if currentChildIndex == t.ChildIndex || t.ChildIndex == -1 {
						if fieldNode.Type == html.TextNode {
							fieldString, err = ExtractStringRegex(&t.RegexExtract, fieldNode.Data)
							if err == nil {
								fieldString = strings.TrimSpace(fieldString)
								if t.MaxLength > 0 && t.MaxLength < len(fieldString) {
									fieldString = fieldString[:t.MaxLength] + "..."
								}
								return fieldString, nil
							} else if t.ChildIndex != -1 {
								// only in case we do not (ab)use the regex to search across all children
								// we want to return the err. Also, we still return the fieldString as
								// this might be useful for narrowing down the reason for the error.
								return fieldString, err
							}
						}
					}
					fieldNode = fieldNode.NextSibling
					currentChildIndex++
				}
			}
		} else {
			// WRONG
			// It could be the case that there are multiple nodes that match the selector
			// and we don't want the attr of the first node...
			fieldString = fieldSelection.AttrOr(t.Attr, "")
		}
	}
	// automatically trimming whitespaces might be confusing in some cases...
	fieldString = strings.TrimSpace(fieldString)
	fieldString, err = ExtractStringRegex(&t.RegexExtract, fieldString)
	if err != nil {
		return fieldString, err
	}
	if t.MaxLength > 0 && t.MaxLength < len(fieldString) {
		fieldString = fieldString[:t.MaxLength] + "..."
	}
	return fieldString, nil
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
