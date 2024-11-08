package search

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

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
	Aggregations   map[string]interface{} `json:"aggregations"`
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
	Field       []string `json:"field"`
	Filters     map[string]map[string]interface{} `json:"filters"`
}

type ArrayFieldQuery struct {
	QueryString []string `json:"query"`
	Field       []string `json:"field"`
	Filters     map[string]map[string]interface{} `json:"filters"`
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

	queryString := buildDoiQuery(query)

	urlPath := fmt.Sprintf(
		"%s/search?%s&resultType=core&format=json&pageSize=100",
		os.Getenv("PMC_URL"),
		queryString,
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

	queryString := buildQueryString(query)

	urlPath := fmt.Sprintf(
		"%s/search?%s&resultType=core&format=json&pageSize=100",
		os.Getenv("PMC_URL"),
		queryString,
	)

	respBody := getPMC(urlPath)

	var result PMCCoreResponse
	json.Unmarshal(respBody, &result)

	aggs := calculateAggregations(result)
	result.Aggregations = aggs

	c.JSON(http.StatusOK, result)
}

// ArrayFieldSearch searches the EuropePMC articles API for papers where the given 
// query strings appears in the given field (e.g. "ABSTRACT", "METHODS", "SUPPL").
// Returns results as an array of PaperCore.
func ArrayFieldSearch(c *gin.Context) {
	var queryArray ArrayFieldQuery
	if err := c.BindJSON(&queryArray); err != nil {
		return
	}

	allResults := PMCCoreResponse{
		ResultList: map[string][]PaperCore{
			"result": {},
		},
	}
	maxResults := 100

	resultChannel := make(chan PMCCoreResponse)
	var wg sync.WaitGroup

	for _, query := range(queryArray.QueryString) {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			resultChannel <- epmcFieldQuey(query, queryArray)
		}(query)
	}
	
	go func() {
		wg.Wait()
		close(resultChannel)
	}()

	results := [][]PaperCore{}
	for r := range(resultChannel) {
		results = append(results, r.ResultList["result"])
	}
	shuffledResults := shuffleArrays(results)
	if len(shuffledResults) < maxResults {
		allResults.ResultList["result"] = shuffledResults
	} else {
		allResults.ResultList["result"] = shuffledResults[0:maxResults]
	}

	aggs := calculateAggregations(allResults)
	allResults.Aggregations = aggs
	allResults.HitCount = len(allResults.ResultList["result"])

	c.JSON(http.StatusOK, allResults)
}

func epmcFieldQuey(query string, queryArray ArrayFieldQuery) PMCCoreResponse {
	singleFieldQuery := FieldQuery{
		QueryString: query,
		Field: queryArray.Field,
		Filters: queryArray.Filters,
	}
	queryString := buildQueryString(singleFieldQuery)

	urlPath := fmt.Sprintf(
		"%s/search?%s&resultType=core&format=json&pageSize=100",
		os.Getenv("PMC_URL"),
		queryString,
	)

	respBody := getPMC(urlPath)

	var result PMCCoreResponse
	json.Unmarshal(respBody, &result)

	return result
} 

// getPMC queries the EuropePMC articles API using the given urlPath.
func getPMC(urlPath string) []byte {
	req, err := http.NewRequest("GET", urlPath, strings.NewReader(""))
	if err != nil {
		slog.Info(fmt.Sprintf("Failed to build EPMC query with: %s", err.Error()))
	}
	req.Header.Add("Content-Type", "application/json")

	response, err := Client.Do(req)
	if err != nil {
		slog.Info(fmt.Sprintf("Failed to execute EPMC query with: %s", err.Error()))
	}
	defer response.Body.Close()

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to get EPMC response with: %s", err.Error()))
	}

	return respBody
}

// extractDOI attempts to extract a doi number (starting "10.") from the doi string.
func extractDOI(doi string) string {
	startInd := strings.Index(doi, "10")
	if startInd == -1 {
		slog.Debug(fmt.Sprintf("String is not a valid doi: %s", doi))
		return doi
	}
	endInd := len(doi)
	doiNum := doi[startInd:endInd]
	doiNum = strings.Replace(doiNum, "(", "\\(", -1)
	doiNum = strings.Replace(doiNum, ")", "\\)", -1)

	return doiNum
}

func buildDoiQuery(query Query) string {
	doi := extractDOI(query.QueryString)
	queryString := fmt.Sprintf("query=(DOI:%s)", doi)

	_, ok := query.Filters["paper"]
	if ok {
		filterString := getFilters(query.Filters)
		queryString = fmt.Sprintf("%s%%20AND%%20%s", queryString, filterString)
		return queryString
	} else {
		return queryString
	}
}

func buildQueryString(query FieldQuery) string {
	queryString := "query=("
	queryFormatted := strings.Replace(query.QueryString, " ", "%20", -1)
	for i, fieldString := range(query.Field) {
		if (i == (len(query.Field) - 1)) {
			queryString = fmt.Sprintf(
				"%s%s:%s)%%20AND%%20", queryString, fieldString, queryFormatted,
			)
		} else {
			queryString = fmt.Sprintf(
				"%s%s:%s%%20OR%%20", queryString, fieldString, queryFormatted,
			)
		}
	}
	_, ok := query.Filters["paper"]
	if ok {
		filterString := getFilters(query.Filters)
		fullString := fmt.Sprintf("%s%s", queryString, filterString)
		return fullString
	} else {
		return queryString
	}
}

func getFilters(filters map[string]map[string]interface{}) string {
	var queryString string
	var filterType []string
	var allFilterType string
	if val, ok := filters["paper"]["publicationDate"]; ok {
		filterDate := fmt.Sprintf(
			"PUB_YEAR:[%s%%20TO%%20%s]", 
			val.([]interface{})[0].(string),
			val.([]interface{})[1].(string),
		)
		queryString = filterDate
	}
	if val, ok := filters["paper"]["publicationType"]; ok {
		for _, t := range(val.([]interface{})) {
			str := publicationTypeFilter(t.(string))
			filterType = append(filterType, str)
		}
		allFilterType = strings.Join(filterType, "%20OR%20")
		if (queryString != "") {
			queryString = fmt.Sprintf("%s%%20AND%%20%s", queryString, allFilterType)
		} else {
			queryString = allFilterType
		}
	}
	return queryString
}

func publicationTypeFilter(pubType string) string {
	var filterStr string
	if (pubType == "Research articles") {
		filterStr = strings.Replace(
			"(((SRC:MED OR SRC:PMC OR SRC:AGR OR SRC:CBA) NOT (PUB_TYPE:\"Review\")))", " ", "%20", -1,
		)
	} else if (pubType == "Review articles") {
		filterStr = "PUB_TYPE:REVIEW"
	} else if (pubType == "Preprints") {
		filterStr = "SRC:PPR"
	} else if (pubType == "Books and documents") {
		filterStr = "HAS_BOOK:Y"
	} else {
		slog.Debug(fmt.Sprintf("Unknown filter option: %s", pubType))
	}

	return filterStr
}

func calculateAggregations(results PMCCoreResponse) gin.H {
	aggregations := make(map[string]interface{})
	minDate := time.Now()
	maxDate, _ := time.Parse("2006", "1900")
	for _, res := range(results.ResultList["result"]) {
		d, err := time.Parse("2006", res.PubYear)
		if err != nil {
			slog.Info(fmt.Sprintf("Failed to convert year to date: %s", res.PubYear))
			continue
		}
		if (d.Before(minDate)) {
			minDate = d
		}
		if (d.After(maxDate)) {
			maxDate = d
		}

	}
	aggregations["startDate"] = gin.H{"value_as_string": minDate.Format(time.RFC3339)}
	aggregations["endDate"] = gin.H{"value_as_string": maxDate.Format(time.RFC3339)}
	return aggregations
}

func reverse(str string) (result string) { 
    for _, v := range str { 
        result = string(v) + result 
    } 
    return
}

// shuffleArrays takes an array of arrays of generic type and
// returns a single array where the elements are the first of each input array,
// the second of each input array, etc until all input arrays are exhausted.
func shuffleArrays[A any](results [][]A) (shuffledResults []A) {
	lengths := make([]int, len(results))
	for i, r := range(results) {
		lengths[i] = len(r)
	}
	sortedLengths := append([]int{}, lengths...)
	sort.Ints(sortedLengths)
	maximum := sortedLengths[len(sortedLengths)-1]

	for j := 0; j < maximum; j++ {
		for k := 0; k < len(results); k++ {
			if (j < lengths[k]) {
				shuffledResults = append(shuffledResults, results[k][j])
			}
		}
	}
	return
}
