package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

// LocalConfig is for passing response related data between parent and children
// Elements
type LocalConfig struct {
	scheme string
	host   string
	path   string
}

// Config defines the overall structure of the scraper configuration.
// Values will be taken from a config yml file or environment variables
// or both.
type Config struct {
	Writer   output.WriterConfig `json:"writer"`
	Elements []Element           `json:"scrapers"`
	Global   GlobalConfig        `json:"global"`
}

// NewConfig Reads the YML config into config
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
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal("Err at closing YML config file: ", err)
		}
	}(file)
	d := json.NewDecoder(file)
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	//fmt.Println(config)
	return &config, nil
}

// RegexConfig is used for extracting a substring from a string based on the
// given Exp and Index
type RegexConfig struct {
	Exp   string `json:"exp"`
	Index int    `json:"index"`
}

// A Filter is used to filter certain items from the result list
type Filter struct {
	Field string `json:"field"`
	Regex string `json:"regex"`
	Match bool   `json:"match"`
}

// Element contains all the necessary config parameters and structs needed
// to extract the desired information from a website
type Element struct {
	Id                  string   `json:"id"`
	URL                 string   `json:"url"`
	ExcludeWithSelector []string `json:"exclude_with_selector"`
	CanBeEmpty          bool     `json:"can_be_empty"` // applies to text, url
	Hide                bool     `json:"hide"`         // applies to text, url, date
	Multiple            bool     `json:"multiple"`
	Delay               int      `json:"delay"`

	Type           string `json:"type"` // can currently be text, url or date
	Selector       string `json:"selector"`
	PaginationType string `json:"paginationType"`

	Selectors       []Element `json:"selectors,omitempty"`
	ParentSelectors []string  `json:"parentSelectors,omitempty"`
	IsParent        bool      `json:"isParent"`

	//Actions []Action `json:actions`

	//Fields struct {
	//	Static  []StaticField `json:"statics"`
	//	Element []Element     `json:"elements"`
	//} `json:"fields"`
	//Filters   []Filter `json:"filters"`
	//Paginator struct {
	//	Location ElementLocation `json:"location"`
	//	MaxPages int             `json:"max_pages"`
	//}
}

type Selector interface {
	CallRecursiveElements(*GlobalConfig, string) ([]map[string]interface{}, error)
	HasParent(string) bool
}

func (ele *Element) HasSpecificParent(parentId string) bool {
	for _, parent := range ele.ParentSelectors {
		//fmt.Printf("Comparing between %s and %s...\n", parentId, parent)
		if parent == parentId {
			return true
		}
	}
	return false
}

func (ele *Element) IsAParent() bool {
	//for _S
	//for _, parent := range ele.ParentSelectors {
	//	if parent.Id == ele.Id {
	//		return true
	//	}
	//}
	//return false
	return ele.IsParent
}

func (ele *Element) Call(globalConfig *GlobalConfig, localConfig *LocalConfig, callThis string, parentSelection *goquery.Selection, root *Element) ([]map[string]interface{}, error) {
	var items []map[string]interface{}
	// Premise: Am I Root or NOT
	isRoot := len(ele.Selectors) > 0
	if isRoot {
		var res *http.Response
		var err error
		if callThis == "" {
			res, err = utils.FetchUrl(ele.URL, "")
		} else {
			res, err = utils.FetchUrl(callThis, "")
		}
		scheme := res.Request.URL.Scheme
		host := res.Request.URL.Host
		path := res.Request.URL.Path
		localConfig = &LocalConfig{
			scheme, host, path,
		}
		// 0. turn said response into a goqueryDoc, then a selection and throw away the url
		parentDoc, err := goquery.NewDocumentFromReader(res.Body)
		parentSelection = parentDoc.Selection
		if err != nil {
			return items, err
		}
		root = ele
	} else {

	}

	// 1. Recursion
	for _, sel := range root.Selectors {
		//fmt.Println(sel)
		tempStorage := make(map[string]interface{})
		// Which ones of my child should do work?
		if sel.HasSpecificParent(ele.Id) {
			// Scenario A: My child is also a parent
			if sel.Type == "SelectorPagination" {
				fmt.Println("PAGINATING AT " + sel.Id)
				paginationRoster := make(map[string]interface{})
				extractField(&sel, paginationRoster, parentSelection, ele.URL, localConfig.host, localConfig.scheme, localConfig.path)
				linkList, _ := paginationRoster[sel.Id].([]string)

				for _, link := range linkList {
					pagResult, _ := ele.Call(globalConfig, localConfig, link, nil, root)
					items = append(items, pagResult...)
				}
			} else {
				if sel.IsAParent() {
					//fmt.Printf("My child %s is a parent!", sel.Id)
					// Scenario A1: Are you going to paginate?
					// Scenario A2: Not pagination eh?
					childResult, err := sel.Call(nil, localConfig, "", parentSelection, ele)
					if err != nil {
						log.Fatalf("OH NO CHILD %d fucked up", sel.Id)
					}
					items = append(items, childResult...)
					sel.DoWork(tempStorage, parentSelection, ele.URL, localConfig)

				} else { // Scenario B: My child is NOT a parent
					//fmt.Printf("My child %s is NOT parent!", sel.Id)
					sel.DoWork(tempStorage, parentSelection, ele.URL, localConfig)
				}
			}
			if len(tempStorage) > 0 {
				items = append(items, tempStorage)
			}
		}
	}
	return items, nil
}

func (ele *Element) DoWork(
	event map[string]interface{},
	s *goquery.Selection,
	baseURL string,
	localConfig *LocalConfig) error {
	err := extractField(ele, event, s, baseURL, localConfig.host, localConfig.scheme, localConfig.path)
	return err
}

// Ah, here is the extraction logic!
func extractField(field *Element, event map[string]interface{}, s *goquery.Selection, baseURL string, scheme string, host string, path string) error {
	switch field.Type {
	case "SelectorText", "": // the default, ie when type is not configured, is 'text'
		ts, err := GetTextString(field.Selector, s)
		if err != nil {
			return err
		}
		if !field.CanBeEmpty && len(ts) == 0 {
			return fmt.Errorf("field %s cannot be empty", field.Id)
		}
		event[field.Id] = ts
	case "SelectorLink", "SelectorPagination":
		ts := getURLStrings(field.Selector, s, scheme, host, path)
		fmt.Println(ts)
		event[field.Id] = ts
	case "url":
		url := getURLString(&field.Selector, s, scheme, host, path)
		if url == "" {
			url = baseURL
		}
		event[field.Id] = url
	default:
		return fmt.Errorf("field type '%s' does not exist", field.Type)
	}
	return nil
}

func getURLStrings(e string, s *goquery.Selection, scheme string, host string, path string) []string {
	targetAttr := "href"
	fieldSelection := s.Find(e)
	rawUrls := fieldSelection.Map(func(i int, selection *goquery.Selection) (url string) {
		rawUrl, _ := selection.Attr(targetAttr)
		urlVal := strings.TrimSpace(rawUrl)
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
		return url
	})
	return rawUrls
}

func getURLString(e *string, s *goquery.Selection, scheme string, host string, path string) string {
	var urlVal, url string
	targetAttr := "href"
	if *e == "" {
		urlVal = s.AttrOr(targetAttr, "")
	} else {
		fieldSelection := s.Find(*e)
		urlVal, _ = fieldSelection.Attr(targetAttr)
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
