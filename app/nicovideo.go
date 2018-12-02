package app

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	baseURL = "http://api.search.nicovideo.jp/api/v2/video/contents/search"
)

type NicovideoAPIResponse struct {
	Meta Meta   `json:"meta"`
	Data []Data `json:"data"`
}

type Meta struct {
	Status     int    `json:"status"`
	TotalCount int    `json:"totalCount"`
	ID         string `json:"id"`
}

type Data struct {
	StartTime      string `json:"startTime"`
	MylistCounter  int    `json:"mylistCounter"`
	ViewCounter    int    `json:"viewCounter"`
	ContentID      string `json:"contentID"`
	Title          string `json:"title"`
	CommentCounter int    `json:"commentCounter"`
}

type NicovideoAPIClient struct {
	baseURL    string
	appName    string
	userAgent  string
	HTTPClient *http.Client
}

func NewNicovideoAPIClient(appName string, userAgent string) *NicovideoAPIClient {
	return &NicovideoAPIClient{
		baseURL:    baseURL,
		appName:    appName,
		userAgent:  userAgent,
		HTTPClient: http.DefaultClient,
	}
}

func (c *NicovideoAPIClient) Get(offset int) (*http.Request, *http.Response) {
	location, _ := time.LoadLocation("Asia/Tokyo")
	sevenDaysAgo := time.Now().
		In(location).
		AddDate(0, 0, -7).
		Format("2006-01-02T15:04:05+09:00")

	urlValues := url.Values{}
	urlValues.Add("q", "RTA biim")
	urlValues.Add("targets", "tags")
	urlValues.Add("filters[categoryTags][0]", "ゲーム")
	urlValues.Add("filters[startTime][gte]", sevenDaysAgo)
	urlValues.Add("fields", "contentId,title,viewCounter,mylistCounter,commentCounter,startTime")
	urlValues.Add("_limit", "100")
	urlValues.Add("_offset", strconv.Itoa(offset))
	urlValues.Add("_sort", "-viewCounter")
	urlValues.Add("_context", c.appName)
	url := baseURL + "?" + urlValues.Encode()

	req, _ := http.NewRequest("GET", url, nil)
	if len(c.userAgent) > 0 {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, _ := c.HTTPClient.Do(req)

	// TODO: ステータスが200以外だった場合の処理
	// resp.StatusCode != http.StatusOK

	return req, resp
}

func (c *NicovideoAPIClient) Parse(resp *http.Response) NicovideoAPIResponse {
	var responseJSON NicovideoAPIResponse
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(bodyBytes, &responseJSON)

	return responseJSON
}
