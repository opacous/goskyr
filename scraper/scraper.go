package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/jakopako/goskyr/output"
	"github.com/jakopako/goskyr/utils"
)

// GlobalConfig is used for storing global configuration parameters that
// are needed across all scrapers
type GlobalConfig struct {
	UserAgent string `json:"user-agent"`
}

// Config defines the overall structure of the scraper configuration.
// Values will be taken from a config yml file or environment variables
// or both.
type Config struct {
	Writer   output.WriterConfig `json:"writer"`
	Elements []Element           `json:"scrapers"`
	Global   GlobalConfig        `json:"global"`
}

// Reads the YML config into config
func NewConfig(configPath string) (*Config, error) {
	var config Config

	err := cleanenv.ReadConfig(configPath, &config)
	if err != nil {
		log.Fatal(err)
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	d := json.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// RegexConfig is used for extracting a substring from a string based on the
// given Exp and Index
type RegexConfig struct {
	Exp   string `json:"exp"`
	Index int    `json:"index"`
}

// A DynamicField contains all the information necessary to scrape
// a dynamic field from a website, ie a field who's value changes
// for each item

// ElementLocation is used to find a specific string in a html document
type ElementLocation struct {
	Selector      string      `json:"selector"`
	NodeIndex     int         `json:"node_index"`
	ChildIndex    int         `json:"child_index"`
	RegexExtract  RegexConfig `json:"regex_extract"`
	Attr          string      `json:"attr"`
	MaxLength     int         `json:"max_length"`
	EntireSubtree bool        `json:"entire_subtree"`
}

// CoveredDateParts is used to determine what parts of a date a
// DateComponent covers
type CoveredDateParts struct {
	Day   bool `json:"day"`
	Month bool `json:"month"`
	Year  bool `json:"year"`
	Time  bool `json:"time"`
}

// A DateComponent is used to find a specific part of a date within
// a html document
type DateComponent struct {
	Covers          CoveredDateParts `json:"covers"`
	ElementLocation ElementLocation  `json:"location"`
	Layout          []string         `json:"layout"`
}

// A StaticField defines a field that has a fixed name and value
// across all scraped items
type StaticField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// A Filter is used to filter certain items from the result list
type Filter struct {
	Field string `json:"field"`
	Regex string `json:"regex"`
	Match bool   `json:"match"`
}

// A Element contains all the necessary config parameters and structs needed
// to extract the desired information from a website

// A "Scraper" is just a Element with a starting URL
type Action interface {
}

type Element struct {
	Name                string          `json:"name"`
	URL                 string          `json:"url"`
	Type                string          `json:"type"` // can currently be text, url or date
	ElementLocation     ElementLocation `json:"location"`
	RecurLocation       ElementLocation `json:"recur_location"`
	ExcludeWithSelector []string        `json:"exclude_with_selector"`
	CanBeEmpty          bool            `json:"can_be_empty"` // applies to text, url
	Hide                bool            `json:"hide"`         // appliess to text, url, date

	Actions []Action `json:jfdisjfdsijfisd`

	Fields struct {
		Static  []StaticField `json:"statics"`
		Element []Element     `json:"elements"`
	} `json:"fields"`
	Filters   []Filter `json:"filters"`
	Paginator struct {
		Location ElementLocation `json:"location"`
		MaxPages int             `json:"max_pages"`
	}
}

type Selector interface {
	// ScrapeNonRecursiveElements() ([]map[string]interface{}, error)
	CallRecursiveElements(*GlobalConfig, string) ([]map[string]interface{}, error)
	// doWork() ([]map[string]interface{}, error)
}

type Complaint interface{}

func (ele Element) CallRecursiveElements(globalConfig *GlobalConfig, callThis string) ([]map[string]interface{}, error) {
	var items []map[string]interface{}
	pageURL := ""
	if callThis != "" {
		pageURL = callThis
	} else {
		pageURL = ele.URL
	}
	fmt.Println("Starting at: " + pageURL)

	res, err := utils.FetchUrl(pageURL, "")
	if err != nil {
		return items, err
	}

	// #TODO: Make sure the fetcher gives the doc, but also with enough information
	// that the consumer is not starved for the response
	scheme := res.Request.URL.Scheme
	host := res.Request.URL.Host
	path := res.Request.URL.Path

	// 0. turn said response into a goqueryDoc
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return items, err
	}

	// Test: Is this a root level? Or recurv?
	isRoot := ele.URL != ""
	topLvSelector := ""
	if isRoot {
		topLvSelector = ele.ElementLocation.Selector
	} else {
		topLvSelector = ele.RecurLocation.Selector
	}

	doc.Find(topLvSelector).Each(func(i int, s *goquery.Selection) {
		currentItem := make(map[string]interface{})

		// 0. Static Elements First
		for _, sf := range ele.Fields.Static {
			currentItem[sf.Name] = sf.Value
		}

		for _, f := range ele.Fields.Element {
			fmt.Println(f)
			eleIsRecursive := len(f.Fields.Element) != 0
			// fmt.Println(eleIsRecursive)
			// 1.A - Get the dumb elements
			if !eleIsRecursive {
				err = extractField(&f, currentItem, s, ele.URL, scheme, host, path)
				// fmt.Println(currentItem)
				// 2.B - Get the recursive elements
			} else {
				targetUrl := getURLString(&f.ElementLocation, s, scheme, host, path)
				fmt.Println("Lets call recurve at: " + targetUrl)
				// Range, recursiveley call each of the recursive seleectors (usually as dynamic)
				recurvItem, _ := f.CallRecursiveElements(globalConfig, targetUrl)
				fmt.Println(recurvItem)
				// TODO: Better parent-child id/naming scheme and flow for adding
				// results from recursion to the original pile of results
				items = append(items, recurvItem...)
			}
		}
		items = append(items, currentItem)
	})

	return items, nil
}

func (c *Element) filterItem(item map[string]interface{}) (bool, error) {
	nrMatchTrue := 0
	filterMatchTrue := false
	filterMatchFalse := true
	for _, filter := range c.Filters {
		regex, err := regexp.Compile(filter.Regex)
		if err != nil {
			return false, err
		}
		if fieldValue, found := item[filter.Field]; found {
			if filter.Match {
				nrMatchTrue++
				if regex.MatchString(fmt.Sprint(fieldValue)) {
					filterMatchTrue = true
				}
			} else {
				if regex.MatchString(fmt.Sprint(fieldValue)) {
					filterMatchFalse = false
				}
			}
		}
	}
	if nrMatchTrue == 0 {
		filterMatchTrue = true
	}
	return filterMatchTrue && filterMatchFalse, nil
}

func (c *Element) removeHiddenFields(item map[string]interface{}) map[string]interface{} {
	for _, f := range c.Fields.Element {
		if f.Hide {
			delete(item, f.Name)
		}
	}
	return item
}

// Ahhh, here is the extraction logic!
func extractUrlEle(field *Element, event map[string]interface{}, s *goquery.Selection, baseURL string, scheme string, host string, path string) error {
	url := getURLString(&field.ElementLocation, s, scheme, host, path)
	if url == "" {
		url = baseURL
	}
	event[field.Name] = url
	return nil
}

// Ahhh, here is the extraction logic!
func extractField(field *Element, event map[string]interface{}, s *goquery.Selection, baseURL string, scheme string, host string, path string) error {
	switch field.Type {
	case "text", "": // the default, ie when type is not configured, is 'text'
		ts, err := GetTextString(&field.ElementLocation, s)
		if err != nil {
			return err
		}
		if !field.CanBeEmpty && ts == "" {
			return fmt.Errorf("field %s cannot be empty", field.Name)
		}
		event[field.Name] = ts
	case "url":
		url := getURLString(&field.ElementLocation, s, scheme, host, path)
		if url == "" {
			url = baseURL
		}
		event[field.Name] = url
	default:
		return fmt.Errorf("field type '%s' does not exist", field.Type)
	}
	return nil
}

func getURLString(e *ElementLocation, s *goquery.Selection, scheme string, host string, path string) string {
	var urlVal, url string
	if e.Attr == "" {
		// set attr to the default if not set
		e.Attr = "href"
	}
	if e.Selector == "" {
		urlVal = s.AttrOr(e.Attr, "")
	} else {
		fieldSelection := s.Find(e.Selector)
		if len(fieldSelection.Nodes) > e.NodeIndex {
			fieldNode := fieldSelection.Get(e.NodeIndex)
			for _, a := range fieldNode.Attr {
				if a.Key == e.Attr {
					urlVal = a.Val
					break
				}
			}
		}
	}

	if urlVal == "" {
		return ""
	} else if strings.HasPrefix(urlVal, "http") {
		url = urlVal
	} else if strings.HasPrefix(urlVal, "?") {
		url = fmt.Sprintf("%s://%s%s%s", scheme, host, path, urlVal)
	} else {
		baseURL := fmt.Sprintf("%s://%s", scheme, host)
		if !strings.HasPrefix(urlVal, "/") {
			baseURL = baseURL + "/"
		}
		url = fmt.Sprintf("%s%s", baseURL, urlVal)
	}

	url = strings.TrimSpace(url)
	return url
}
