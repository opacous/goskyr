// package scraper
package scraper

// import (
// 	"fmt"
// 	"log"
// 	"net/http"
// 	"os"
// 	"regexp"
// 	"strings"

// 	"github.com/PuerkitoBio/goquery"
// 	"github.com/ilyakaznacheev/cleanenv"
// 	"github.com/jakopako/goskyr/output"
// 	"github.com/jakopako/goskyr/utils"
// 	"gopkg.in/yaml.v2"
// )

// // GlobalConfig is used for storing global configuration parameters that
// // are needed across all scrapers
// type GlobalConfig struct {
// 	UserAgent string `yaml:"user-agent"`
// }

// // Config defines the overall structure of the scraper configuration.
// // Values will be taken from a config yml file or environment variables
// // or both.
// type Config struct {
// 	Writer   output.WriterConfig `yaml:"writer"`
// 	Scrapers []Scraper           `yaml:"scrapers"`
// 	Global   GlobalConfig        `yaml:"global"`
// }

// // Reads the YML config into config
// func NewConfig(configPath string) (*Config, error) {
// 	var config Config

// 	err := cleanenv.ReadConfig(configPath, &config)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	file, err := os.Open(configPath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer file.Close()
// 	d := yaml.NewDecoder(file)
// 	if err := d.Decode(&config); err != nil {
// 		return nil, err
// 	}
// 	return &config, nil
// }

// // RegexConfig is used for extracting a substring from a string based on the
// // given Exp and Index
// type RegexConfig struct {
// 	Exp   string `yaml:"exp"`
// 	Index int    `yaml:"index"`
// }

// // ElementLocation is used to find a specific string in a html document
// type ElementLocation struct {
// 	Selector      string      `yaml:"selector"`
// 	NodeIndex     int         `yaml:"node_index"`
// 	ChildIndex    int         `yaml:"child_index"`
// 	RegexExtract  RegexConfig `yaml:"regex_extract"`
// 	Attr          string      `yaml:"attr"`
// 	MaxLength     int         `yaml:"max_length"`
// 	EntireSubtree bool        `yaml:"entire_subtree"`
// }

// // CoveredDateParts is used to determine what parts of a date a
// // DateComponent covers
// type CoveredDateParts struct {
// 	Day   bool `yaml:"day"`
// 	Month bool `yaml:"month"`
// 	Year  bool `yaml:"year"`
// 	Time  bool `yaml:"time"`
// }

// // A DateComponent is used to find a specific part of a date within
// // a html document
// type DateComponent struct {
// 	Covers          CoveredDateParts `yaml:"covers"`
// 	ElementLocation ElementLocation  `yaml:"location"`
// 	Layout          []string         `yaml:"layout"`
// }

// // A StaticField defines a field that has a fixed name and value
// // across all scraped items
// type StaticField struct {
// 	Name  string `yaml:"name"`
// 	Value string `yaml:"value"`
// }

// // A DynamicField contains all the information necessary to scrape
// // a dynamic field from a website, ie a field who's value changes
// // for each item
// type DynamicField struct {
// 	Name string `yaml:"name"`
// 	Type string `yaml:"type"` // can currently be text, url or date
// 	// If a field can be found on a subpage the following variable has to contain a field name of
// 	// a field of type 'url' that is located on the main page.
// 	ElementLocation ElementLocation `yaml:"location"`
// 	OnSubpage       string          `yaml:"on_subpage"`    // applies to text, url, date
// 	CanBeEmpty      bool            `yaml:"can_be_empty"`  // applies to text, url
// 	Components      []DateComponent `yaml:"components"`    // applies to date
// 	DateLocation    string          `yaml:"date_location"` // applies to date
// 	DateLanguage    string          `yaml:"date_language"` // applies to date
// 	Hide            bool            `yaml:"hide"`          // appliess to text, url, date
// 	// URL                 string          `yaml:"url"`
// 	// Item                string          `yaml:"item"` // Equivalent to selector => ".col-16-4 .icon"
// 	// ExcludeWithSelector []string        `yaml:"exclude_with_selector"`
// 	Fields struct {
// 		Static  []StaticField  `yaml:"static"`
// 		Dynamic []DynamicField `yaml:"dynamic"`
// 	} `yaml:"fields"`
// 	// Filters   []Filter `yaml:"filters"`
// 	// Paginator struct {
// 	// 	Location ElementLocation `yaml:"location"`
// 	// 	MaxPages int             `yaml:"max_pages"`
// 	// }
// }

// // A Filter is used to filter certain items from the result list
// type Filter struct {
// 	Field string `yaml:"field"`
// 	Regex string `yaml:"regex"`
// 	Match bool   `yaml:"match"`
// }

// // A Scraper contains all the necessary config parameters and structs needed
// // to extract the desired information from a website
// type Scraper struct {
// 	Name                string   `yaml:"name"`
// 	URL                 string   `yaml:"url"`
// 	Item                string   `yaml:"item"`
// 	ExcludeWithSelector []string `yaml:"exclude_with_selector"`
// 	Fields              struct {
// 		Static  []StaticField  `yaml:"static"`
// 		Dynamic []DynamicField `yaml:"dynamic"`
// 	} `yaml:"fields"`
// 	Filters   []Filter `yaml:"filters"`
// 	Paginator struct {
// 		Location ElementLocation `yaml:"location"`
// 		MaxPages int             `yaml:"max_pages"`
// 	}
// }

// type Selector interface {
// 	// ScrapeNonRecursiveElements() ([]map[string]interface{}, error)
// 	CallRecursiveElements(parentDoc *goquery.Document) ([]map[string]interface{}, error)
// 	// doWork() ([]map[string]interface{}, error)
// }

// type Complaint interface{}

// func (selector DynamicField) CallRecursiveElements(parentDoc *goquery.Document) ([]map[string]interface{}, error) {
// 	var items []map[string]interface{}

// 	// -1. Get the response at URL
// 	// res, err := utils.FetchUrl(pageURL, "")
// 	// if err != nil {
// 	// 	return items, err
// 	// }

// 	// 0. turn said response into a goqueryDoc
// 	doc, err := goquery.NewDocumentFromReader(res.Body)
// 	if err != nil {
// 		return items, err
// 	}

// 	doc.Find(selector.Item).Each(func(i int, s *goquery.Selection) {
// 		currentItem := make(map[string]interface{})
// 		// Range, recursively call each of the recursive selectors (usually as dynamic)
// 		subpagesResp := make(map[string]*Complaint)
// 		subpagesBody := make(map[string]*goquery.Document)
// 		for _, f := range selector.Fields.Dynamic {
// 			if f.OnSubpage != "" {
// 				// check whether we fetched the page already
// 				subpageURL := fmt.Sprint(currentItem[f.OnSubpage])
// 				resSub, found := subpagesResp[subpageURL]
// 				if !found {
// 					resSub, err = utils.FetchUrl(subpageURL, "")
// 					if err != nil {
// 						log.Printf("%s ERROR: %v. Skipping item %v.", selector.Name, err, currentItem)
// 						return
// 					}
// 					if resSub.StatusCode != 200 {
// 						log.Printf("%s ERROR: status code error: %d %s. Skipping item %v.", selector.Name, res.StatusCode, res.Status, currentItem)
// 						return
// 					}
// 					subpagesResp[subpageURL] = resSub
// 					docSub, err := goquery.NewDocumentFromReader(resSub.Body)

// 					if err != nil {
// 						log.Printf("%s ERROR: error while reading document: %v. Skipping item %v", selector.Name, err, currentItem)
// 						return
// 					}
// 					subpagesBody[subpageURL] = docSub
// 				}
// 				err = extractField(&f, currentItem, subpagesBody[subpageURL].Selection, selector.URL, resSub)
// 				if err != nil {
// 					log.Printf("%s ERROR: error while parsing field %s: %v. Skipping item %v.", selector.Name, f.Name, err, currentItem)
// 					return
// 				}
// 			}
// 		}
// 		// close all the subpages
// 		for _, resSub := range subpagesResp {
// 			resSub.Body.Close()
// 		}
// 		items = append(items, currentItem)
// 	})

// 	for _, el := range selector.Fields.Dynamic {
// 		recurItem, _ := el.CallRecursiveElements()
// 		items = append(items, recurItem...)
// 	}

// 	return items, nil
// }

// func (scraper Scraper) ScrapeNonRecursiveElements() ([]map[string]interface{}, error) {
// 	var results []map[string]interface{}
// 	pageURL := scraper.URL

// 	// -1. Get the response at URL
// 	res, err := utils.FetchUrl(pageURL, "")
// 	if err != nil {
// 		return results, err
// 	}

// 	// 0. turn said response into a goqueryDoc
// 	doc, err := goquery.NewDocumentFromReader(res.Body)
// 	if err != nil {
// 		return results, err
// 	}

// 	doc.Find(scraper.Item).Each(func(i int, s *goquery.Selection) {
// 		//fmt.Println(s.Html())
// 		// This is usually empty, so I guess this just skips each selection if
// 		// anything in the selection is	 a match for anything in []excludeWithSelector{...}
// 		for _, excludeSelector := range scraper.ExcludeWithSelector {
// 			if s.Find(excludeSelector).Length() > 0 || s.Is(excludeSelector) {
// 				return
// 			}
// 		}

// 		// Add in static fields
// 		currentNonRecursiveItems := make(map[string]interface{})
// 		for _, sf := range scraper.Fields.Static {
// 			currentNonRecursiveItems[sf.Name] = sf.Value
// 		}

// 		// handle all non-recursive fields on the current/root page
// 		for _, f := range scraper.Fields.Dynamic {
// 			if f.OnSubpage == "" {
// 				err := extractField(&f, currentNonRecursiveItems, s, scraper.URL, res)
// 				fmt.Println(currentNonRecursiveItems)
// 				if err != nil {
// 					log.Printf("%s ERROR: error while parsing field %s: %v. Skipping item %v.", scraper.Name, f.Name, err, currentNonRecursiveItems)
// 					return
// 				}
// 			}
// 		}
// 		results = append(results, currentNonRecursiveItems)
// 	})

// 	return results, nil
// }

// func (scraper Scraper) CallRecursiveElements(globalConfig *GlobalConfig) ([]map[string]interface{}, error) {
// 	var items []map[string]interface{}
// 	pageURL := scraper.URL

// 	// -1. Get the response at URL
// 	res, err := utils.FetchUrl(pageURL, "")
// 	if err != nil {
// 		return items, err
// 	}

// 	// 0. turn said response into a goqueryDoc
// 	doc, err := goquery.NewDocumentFromReader(res.Body)
// 	if err != nil {
// 		return items, err
// 	}

// 	doc.Find(scraper.Item).Each(func(i int, s *goquery.Selection) {
// 		currentItem := make(map[string]interface{})
// 		// Range, recursively call each of the recursive selectors (usually as dynamic)
// 		subpagesResp := make(map[string]*http.Response)
// 		subpagesBody := make(map[string]*goquery.Document)
// 		for _, f := range scraper.Fields.Dynamic {
// 			if f.OnSubpage != "" && f.OnSubpage != "url" {
// 				subpageURL := fmt.Sprint(currentItem[f.OnSubpage])
// 				resSub, found := subpagesResp[subpageURL]
// 				// check whether we fetched the page already
// 				if !found {
// 					resSub, err = utils.FetchUrl(subpageURL, globalConfig.UserAgent)
// 					if err != nil {
// 						log.Printf("%s ERROR: %v. Skipping item %v.", scraper.Name, err, currentItem)
// 						return

// 					subpagesResp[subpageURL] = resSub
// 					docSub, err := goquery.NewDocumentFromReader(resSub.Body)

// 					if err != nil {
// 						log.Printf("%s ERROR: error while reading document: %v. Skipping item %v", scraper.Name, err, currentItem)
// 						return
// 					}
// 					subpagesBody[subpageURL] = docSub
// 				}
// 				err = extractField(&f, currentItem, subpagesBody[subpageURL].Selection, scraper.URL, resSub)
// 				if err != nil {
// 					log.Printf("%s ERROR: error while parsing field %s: %v. Skipping item %v.", scraper.Name, f.Name, err, currentItem)
// 					return
// 				}
// 			}
// 		}
// 		// close all the subpages
// 		for _, resSub := range subpagesResp {
// 			resSub.Body.Close()
// 		}
// 		items = append(items, currentItem)
// 	})

// 	for _, el := range scraper.Fields.Dynamic {
// 		recurvItem, _ := el.CallRecursiveElements()
// 		items = append(items, recurvItem...)
// 	}

// 	return items, nil
// }

// // GetItems fetches and returns all items from a website according to the
// // Scraper's paramaters, but enables DynamicField recursion w. limited functionalities
// func (c Scraper) SimonGetItems(globalConfig *GlobalConfig) ([]map[string]interface{}, error) {
// 	return c.ScrapeNonRecursiveElements()
// 	// 1. Loop through the first layer, and grab all the non-url dynamic fields
// 	// 2. Once done, _, dyno := range Fields.Dynamic {_GetItems}
// }

// // GetItems fetches and returns all items from a website according to the
// // Scraper's paramaters
// func (c Scraper) GetItems(globalConfig *GlobalConfig) ([]map[string]interface{}, error) {

// 	//Q: What is this items? The end string results? What if we want specific strings within a ele?
// 	var items []map[string]interface{}

// 	pageURL := c.URL
// 	hasNextPage := true
// 	currentPage := 0
// 	for hasNextPage {
// 		res, err := utils.FetchUrl(pageURL, globalConfig.UserAgent)
// 		if err != nil {
// 			return items, err
// 		}

// 		// defer res.Body.Close() // better not defer in a for loop
// 		// if res.StatusCode != 200 {
// 		// 	return items, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)
// 		// }

// 		// OK, so far, all the work is being done on the doc/goquery equivalent of
// 		// the res.body. the Response itself is not being really used
// 		doc, err := goquery.NewDocumentFromReader(res.Body)
// 		if err != nil {
// 			return items, err
// 		}

// 		doc.Find(c.Item).Each(func(i int, s *goquery.Selection) {
// 			// This is usually empty, so I guess this just skips each selection if
// 			// anything in the selection is a match for anything in []excludeWithSelector{...}
// 			for _, excludeSelector := range c.ExcludeWithSelector {
// 				if s.Find(excludeSelector).Length() > 0 || s.Is(excludeSelector) {
// 					return
// 				}
// 			}

// 			// add static fields
// 			currentItem := make(map[string]interface{})
// 			for _, sf := range c.Fields.Static {
// 				currentItem[sf.Name] = sf.Value
// 			}

// 			// handle all fields on the main page
// 			for _, f := range c.Fields.Dynamic {
// 				if f.OnSubpage == "" {
// 					err := extractField(&f, currentItem, s, c.URL, res)
// 					if err != nil {
// 						log.Printf("%s ERROR: error while parsing field %s: %v. Skipping item %v.", c.Name, f.Name, err, currentItem)
// 						return
// 					}
// 				}
// 			}

// 			// handle all fields on subpages
// 			// Q: Slightly confusing about what's going on here...
// 			// we store the *http.Response as value and not the *goquery.Selection
// 			// to still be able to close all the response bodies afterwards
// 			// UPDATE: we also store the *goquery.Document since apparently resSub.Body
// 			// can only be read once.
// 			subpagesResp := make(map[string]*http.Response)
// 			subpagesBody := make(map[string]*goquery.Document)
// 			for _, f := range c.Fields.Dynamic {
// 				if f.OnSubpage != "" {
// 					// check whether we fetched the page already
// 					subpageURL := fmt.Sprint(currentItem[f.OnSubpage])
// 					resSub, found := subpagesResp[subpageURL]
// 					if !found {
// 						resSub, err = utils.FetchUrl(subpageURL, globalConfig.UserAgent)
// 						if err != nil {
// 							log.Printf("%s ERROR: %v. Skipping item %v.", c.Name, err, currentItem)
// 							return
// 						}
// 						if resSub.StatusCode != 200 {
// 							log.Printf("%s ERROR: status code error: %d %s. Skipping item %v.", c.Name, res.StatusCode, res.Status, currentItem)
// 							return
// 						}
// 						subpagesResp[subpageURL] = resSub
// 						docSub, err := goquery.NewDocumentFromReader(resSub.Body)

// 						if err != nil {
// 							log.Printf("%s ERROR: error while reading document: %v. Skipping item %v", c.Name, err, currentItem)
// 							return
// 						}
// 						subpagesBody[subpageURL] = docSub
// 					}
// 					err = extractField(&f, currentItem, subpagesBody[subpageURL].Selection, c.URL, resSub)
// 					if err != nil {
// 						log.Printf("%s ERROR: error while parsing field %s: %v. Skipping item %v.", c.Name, f.Name, err, currentItem)
// 						return
// 					}
// 				}
// 			}
// 			// close all the subpages
// 			for _, resSub := range subpagesResp {
// 				resSub.Body.Close()
// 			}

// 			// check if item should be filtered
// 			filter, err := c.filterItem(currentItem)
// 			if err != nil {
// 				log.Fatalf("%s ERROR: error while applying filter: %v.", c.Name, err)
// 			}
// 			if filter {
// 				currentItem = c.removeHiddenFields(currentItem)
// 				items = append(items, currentItem)
// 			}
// 		})

// 		hasNextPage = false
// 		pageURL = getURLString(&c.Paginator.Location, doc.Selection, res)
// 		if pageURL != "" {
// 			currentPage++
// 			if currentPage < c.Paginator.MaxPages || c.Paginator.MaxPages == 0 {
// 				hasNextPage = true
// 			}
// 		}
// 		res.Body.Close()
// 	}
// 	// TODO: check if the dates make sense. Sometimes we have to guess the year since it
// 	// does not appear on the website. In that case, eg. having a list of events around
// 	// the end of one year and the beginning of the next year we might want to change the
// 	// year of some events because our previous guess was rather naiv. We also might want
// 	// to make this functionality optional. See issue #68

// 	return items, nil
// }

// func (c *Scraper) filterItem(item map[string]interface{}) (bool, error) {
// 	nrMatchTrue := 0
// 	filterMatchTrue := false
// 	filterMatchFalse := true
// 	for _, filter := range c.Filters {
// 		regex, err := regexp.Compile(filter.Regex)
// 		if err != nil {
// 			return false, err
// 		}
// 		if fieldValue, found := item[filter.Field]; found {
// 			if filter.Match {
// 				nrMatchTrue++
// 				if regex.MatchString(fmt.Sprint(fieldValue)) {
// 					filterMatchTrue = true
// 				}
// 			} else {
// 				if regex.MatchString(fmt.Sprint(fieldValue)) {
// 					filterMatchFalse = false
// 				}
// 			}
// 		}
// 	}
// 	if nrMatchTrue == 0 {
// 		filterMatchTrue = true
// 	}
// 	return filterMatchTrue && filterMatchFalse, nil
// }

// func (c *Scraper) removeHiddenFields(item map[string]interface{}) map[string]interface{} {
// 	for _, f := range c.Fields.Dynamic {
// 		if f.Hide {
// 			delete(item, f.Name)
// 		}
// 	}
// 	return item
// }

// // Ahhh, here is the extraction logic!
// func extractField(field *DynamicField, event map[string]interface{}, s *goquery.Selection, baseURL string, res *http.Response) error {
// 	switch field.Type {
// 	case "text", "": // the default, ie when type is not configured, is 'text'
// 		ts, err := GetTextString(&field.ElementLocation, s)
// 		if err != nil {
// 			return err
// 		}
// 		if !field.CanBeEmpty && ts == "" {
// 			return fmt.Errorf("field %s cannot be empty", field.Name)
// 		}
// 		event[field.Name] = ts
// 	// case "url":
// 	// 	url := getURLString(&field.ElementLocation, s, res)
// 	// 	if url == "" {
// 	// 		url = baseURL
// 	// 	}
// 	// 	event[field.Name] = url
// 	case "date":
// 		d, err := GetDate(field, s)
// 		if err != nil {
// 			return err
// 		}
// 		event[field.Name] = d
// 	default:
// 		return fmt.Errorf("field type '%s' does not exist", field.Type)
// 	}
// 	return nil
// }

// func getURLString(e *ElementLocation, s *goquery.Selection, res *http.Response) string {
// 	var urlVal, url string
// 	if e.Attr == "" {
// 		// set attr to the default if not set
// 		e.Attr = "href"
// 	}
// 	if e.Selector == "" {
// 		urlVal = s.AttrOr(e.Attr, "")
// 	} else {
// 		fieldSelection := s.Find(e.Selector)
// 		if len(fieldSelection.Nodes) > e.NodeIndex {
// 			fieldNode := fieldSelection.Get(e.NodeIndex)
// 			for _, a := range fieldNode.Attr {
// 				if a.Key == e.Attr {
// 					urlVal = a.Val
// 					break
// 				}
// 			}
// 		}
// 	}

// 	if urlVal == "" {
// 		return ""
// 	} else if strings.HasPrefix(urlVal, "http") {
// 		url = urlVal
// 	} else if strings.HasPrefix(urlVal, "?") {
// 		url = fmt.Sprintf("%s://%s%s%s", res.Request.URL.Scheme, res.Request.URL.Host, res.Request.URL.Path, urlVal)
// 	} else {
// 		baseURL := fmt.Sprintf("%s://%s", res.Request.URL.Scheme, res.Request.URL.Host)
// 		if !strings.HasPrefix(urlVal, "/") {
// 			baseURL = baseURL + "/"
// 		}
// 		url = fmt.Sprintf("%s%s", baseURL, urlVal)
// 	}

// 	url = strings.TrimSpace(url)
// 	return url
// }
