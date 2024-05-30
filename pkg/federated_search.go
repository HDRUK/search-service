package search

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// PMCCoreResponse represents the data returned by querying EuropePMC's Articles
// API with the result type specified as "core"
type PMCCoreResponse struct {
	Version        string                 `json:"version"`
	HitCount       int                    `json:"hitCount"`
	NextCursorMark string                 `json:"nextCursorMark"`
	NextPageUrl    string                 `json:"nextPageUrl"`
	Request        map[string]interface{} `json:"request"`
	ResultList     map[string][]PaperCore `json:"resultList"`
}

// PaperCore represents the data returned from EuropePMC about each paper from the
// Articles API when the result type "core" is specified
type PaperCore struct {
	ID 						  string				 `json:"id"` 		
	DOI                       string                 `json:"doi"`
	Title                     string                 `json:"title"`
	AuthorString              string                 `json:"authorString"`
	JournalInfo               map[string]interface{} `json:"journalInfo"`
	BookOrReportDetails		  map[string]interface{} `json:"bookOrReportDetails"`
	PubYear                   string                 `json:"pubYear"`
	AbstractText              string                 `json:"abstractText"`
	PubTypeList               map[string]interface{} `json:"pubTypeList"`
	FullTextUrlList           map[string][]PaperUrl  `json:"fullTextUrlList"`
}

// PaperUrl represents the url objects returned from EuropePMC with each paper
type PaperUrl struct {
	Availability     string `json:"availability"`
	AvailabilityCode string `json:"availabilityCode"`
	DocumentStyle    string `json:"documentStyle"`
	Site             string `json:"site"`
	Url              string `json:"url"`
}

type FieldQuery struct {
	QueryString string `json:"query"`
	Field       string `json:"field"`
	Filters     map[string]map[string]interface{} `json:"filters"`
}

type MultiFieldQuery struct {
	QueryString string 	 `json:"query"`
	Fields      []string `json:"fields"`
}

var (
	Client HTTPClient
)

func init() {
	Client = &http.Client{}
}

// DOISearch takes the given candidate doi string, attempts to extract the DOI
// number from it, then searches the EuropePMC articles API for papers 
// matching that doi.
// Returns results in PMCCoreResponse format.
func DOISearch(c *gin.Context) {
	var query Query
	if err := c.BindJSON(&query); err != nil {
		return
	}

	doi := extractDOI(query.QueryString)

	urlPath := fmt.Sprintf(
		"%s/search?query=%s:%s&resultType=core&format=json&pageSize=100",
		os.Getenv("PMC_URL"),
		"DOI",
		url.QueryEscape(doi),
	)

	respBody := getPMC(urlPath)

	var result PMCCoreResponse
	json.Unmarshal(respBody, &result)

	c.JSON(http.StatusOK, result)
}

// FieldSearch searches the EuropePMC articles API for papers where the given 
// query string appears in the given field (e.g. "ABSTRACT", "METHODS", "SUPPL").
// Returns results as an array of PaperCore.
func FieldSearch(c *gin.Context) {
	var query FieldQuery
	if err := c.BindJSON(&query); err != nil {
		return
	}

	filterString := getFilters(query.Filters)

	urlPath := fmt.Sprintf(
		"%s/search?query=%s:%s%%20AND%%20%s&resultType=core&format=json&pageSize=100",
		os.Getenv("PMC_URL"),
		query.Field,
		url.QueryEscape(query.QueryString),
		filterString,
	)

	fmt.Println(urlPath)

	respBody := getPMC(urlPath)

	var result PMCCoreResponse
	json.Unmarshal(respBody, &result)

	c.JSON(http.StatusOK, result)
}

// MultiFieldSearch searches the EuropePMC articles API for papers where the given 
// query string appears in any of the given fields (e.g. "ABSTRACT", "METHODS", 
// "SUPPL").
// Returns results as an array of PaperCore.
func MultiFieldSearch(c *gin.Context) {
	var query MultiFieldQuery
	if err := c.BindJSON(&query); err != nil {
		return
	}

	var allResults []PaperCore
	var allIDs []string
	for _, field := range(query.Fields) {
		urlPath := fmt.Sprintf(
			"%s/search?query=%s:%s&resultType=core&format=json&pageSize=100",
			os.Getenv("PMC_URL"),
			field,
			url.QueryEscape(query.QueryString),
		)
		
		respBody := getPMC(urlPath)

		var result PMCCoreResponse
		json.Unmarshal(respBody, &result)

		for _, res := range(result.ResultList["result"]) {
			if !(contains(res.ID, allIDs)) {
				allIDs = append(allIDs, res.ID)
				allResults = append(allResults, res)
			}
		}
	}
	c.JSON(http.StatusOK, allResults)
}

// getPMC queries the EuropePMC articles API using the given urlPath.
func getPMC(urlPath string) []byte {
	req, err := http.NewRequest("GET", urlPath, strings.NewReader(""))
	if err != nil {
		fmt.Println(err.Error())
	}
	req.Header.Add("Content-Type", "application/json")

	response, err := Client.Do(req)
	if err != nil {
		fmt.Println(err.Error())
	}
	defer response.Body.Close()

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	return respBody
}

// extractDOI attempts to extract a doi number (starting "10.") from the doi string.
func extractDOI(doi string) string {
	startInd := strings.Index(doi, "10")
	if startInd == -1 {
		fmt.Printf("String is not a valid doi: %s", doi)
		return doi
	}
	endInd := len(doi) - (strings.Index(reverse(doi), "v") + 1)
	if (endInd < startInd) {
		endInd = len(doi)
	}
	doiNum := doi[startInd:endInd]
	doiNum = strings.Replace(doiNum, "(", "\\(", -1)
	doiNum = strings.Replace(doiNum, ")", "\\)", -1)

	return doiNum
}

func getFilters(filters map[string]map[string]interface{}) string {
	var filterString []string
	if val, ok := filters["paper"]["publicationDate"]; ok {
		str := fmt.Sprintf(
			"PUB_YEAR:[%s%%20TO%%20%s]", 
			val.([]interface{})[0], 
			val.([]interface{})[1],
		)
		filterString = append(filterString, str)
	}
	if val, ok := filters["paper"]["publicationType"]; ok {
		for _, t := range(val.([]interface{})) {
			str := fmt.Sprintf("PUB_TYPE:%s", strings.Replace(t.(string), " ", "%20", -1))
			filterString = append(filterString, str)
		}
	}
	queryString := strings.Join(filterString, "%20AND%20")
	return queryString
}

func reverse(str string) (result string) { 
    for _, v := range str { 
        result = string(v) + result 
    } 
    return
}

func contains(item string, collection []string) bool {
    for _, c := range collection {
        if c == item {
            return true
        }
    }
    return false
}
