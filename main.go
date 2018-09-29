package myapp

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type API_Response struct {
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

type Video struct {
	Data
	Tweeted     string
	LastUpdated string
}

const TimeFormatISO8601 = "2006-01-02T15:04:05+09:00"

const PeriodInDay = 7

const ViewCounterThreshold = 10000
const CommentCounterThreshold = 500
const MylistCounterThreshold = 100

const SleepDurationInSec = 1

const TweetLimitAtSameTime = 3

func init() {
	http.HandleFunc("/tasks/main", mainTaskHandler)
}

func mainTaskHandler(_ http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	log.Infof(ctx, "/tasks/main start")

	location, _ := time.LoadLocation("Asia/Tokyo")

	twitterAPI := anaconda.NewTwitterApiWithCredentials(
		os.Getenv("TWITTER_ACCESS_TOKEN"),
		os.Getenv("TWITTER_ACCESS_TOKEN_SECRET"),
		os.Getenv("TWITTER_CONSUMER_KEY"),
		os.Getenv("TWITTER_CONSUMER_SECRET"),
	)
	twitterAPI.HttpClient.Transport = &urlfetch.Transport{Context: ctx}

	// niconico コンテンツ検索APIを叩いてデータを集める
	{
		baseURL := "http://api.search.nicovideo.jp/api/v2/video/contents/search"
		someDaysAgo := time.Now().
			In(location).
			AddDate(0, 0, -PeriodInDay).
			Format(TimeFormatISO8601)
		appName := os.Getenv("APP_NAME")

		urlValues := url.Values{}
		urlValues.Add("q", "RTA biim")
		urlValues.Add("targets", "tags")
		urlValues.Add("filters[categoryTags][0]", "ゲーム")
		urlValues.Add("filters[startTime][gte]", someDaysAgo)
		urlValues.Add("fields", "contentId,title,viewCounter,mylistCounter,commentCounter,startTime")
		urlValues.Add("_limit", "100")
		urlValues.Add("_sort", "-viewCounter")
		urlValues.Add("_context", appName)
		url := baseURL + "?" + urlValues.Encode()
		log.Infof(ctx, "request url: %s", url)

		req, _ := http.NewRequest("GET", url, nil)
		userAgent := os.Getenv("USER_AGENT")
		if len(userAgent) > 0 {
			req.Header.Set("User-Agent", userAgent)
		}

		client := urlfetch.Client(ctx)
		resp, _ := client.Do(req)
		log.Infof(ctx, "response status: %s", resp.Status)

		// TODO: ステータスが200以外だった場合の処理
		// resp.StatusCode != http.StatusOK

		var responseJSON API_Response
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(bodyBytes, &responseJSON)
		log.Infof(ctx, "meta: status=%d id=%s totalCount=%d", responseJSON.Meta.Status, responseJSON.Meta.ID, responseJSON.Meta.TotalCount)
		log.Infof(ctx, "data: count=%d", len(responseJSON.Data))

		for _, data := range responseJSON.Data {
			log.Debugf(ctx, "data: contentId=%s", data.ContentID)

			query := datastore.
				NewQuery("Video").
				Filter("ContentID =", data.ContentID)
			var videos []Video
			keys, err := query.GetAll(ctx, &videos)
			if err != nil {
				panic(err.Error())
			}
			log.Debugf(ctx, "count: %d", len(keys))

			var key *datastore.Key
			var video Video
			now := time.Now().
				In(location).
				Format(TimeFormatISO8601)
			if len(keys) > 0 {
				key = keys[0]
				video = Video{Data: data, Tweeted: videos[0].Tweeted, LastUpdated: now}
			} else {
				key = datastore.NewIncompleteKey(ctx, "Video", nil)
				video = Video{Data: data, Tweeted: "", LastUpdated: now}
			}

			if _, err := datastore.Put(ctx, key, &video); err != nil {
				panic(err.Error())
			}
		}
	}

	// 結果を集計してツイートする
	{
		startTimeFrom := time.Now().
			In(location).
			AddDate(0, 0, -PeriodInDay).
			Format(TimeFormatISO8601)
		query := datastore.
			NewQuery("Video").
			Filter("StartTime >=", startTimeFrom).
			Order("StartTime")
		log.Debugf(ctx, "query conditions: StartTime >= %s", startTimeFrom)

		var videos []Video
		keys, err := query.GetAll(ctx, &videos)
		if err != nil {
			panic(err.Error())
		}
		log.Debugf(ctx, "query result: count=%d", len(keys))

		tweetCount := 0
		for index, video := range videos {
			if len(video.Tweeted) > 0 {
				log.Debugf(ctx, "skip video: contentId=%s tweeted=%s", video.ContentID, video.Tweeted)
				continue
			}

			if video.ViewCounter < ViewCounterThreshold && video.CommentCounter < CommentCounterThreshold && video.MylistCounter < MylistCounterThreshold {
				log.Debugf(ctx, "skip video: contentId=%s viewCounter=%d commentCounter=%d mylistCounter=%d", video.ContentID, video.ViewCounter, video.CommentCounter, video.MylistCounter)
				continue
			}

			printer := message.NewPrinter(language.Japanese)
			status := printer.Sprintf(
				"%d回再生 %dコメント %dマイリスト - %s https://nico.ms/%s #%s #ニコニコ動画",
				video.ViewCounter,
				video.CommentCounter,
				video.MylistCounter,
				video.Title,
				video.ContentID,
				video.ContentID,
			)
			log.Debugf(ctx, "status: %s", status)

			if !appengine.IsDevAppServer() {
				// In production
				tweet, err := twitterAPI.PostTweet(status, nil)
				if err != nil {
					panic(err.Error())
				}
				log.Infof(ctx, "tweet: %s", tweet.FullText)
			}
			tweetCount++

			now := time.Now().
				In(location).
				Format(TimeFormatISO8601)
			video.Tweeted = now
			video.LastUpdated = now
			if _, err := datastore.Put(ctx, keys[index], &video); err != nil {
				panic(err.Error())
			}

			if tweetCount >= TweetLimitAtSameTime {
				break
			}

			time.Sleep(SleepDurationInSec * time.Second)
		}
	}

	log.Infof(ctx, "/tasks/main end")
}
