package myapp

import (
	"net/http"
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

type Video struct {
	Data
	Tweeted     string
	LastUpdated string
}

const (
	timeFormatISO8601       = "2006-01-02T15:04:05+09:00"
	periodInDay             = 7
	viewCounterThreshold    = 10000
	commentCounterThreshold = 500
	mylistCounterThreshold  = 100
	sleepDurationInSec      = 1
	tweetLimitAtSameTime    = 3
)

func init() {
	http.HandleFunc("/tasks/main", mainTaskHandler)
}

func mainTaskHandler(_ http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

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
		nicovideoAPI := NewNicovideoAPIClient(
			os.Getenv("APP_NAME"),
			os.Getenv("USER_AGENT"),
		)
		nicovideoAPI.HTTPClient.Transport = &urlfetch.Transport{Context: ctx}

		req, resp := nicovideoAPI.Get()
		log.Infof(ctx, "request url: %s", req.URL)
		log.Infof(ctx, "response status: %s", resp.Status)
		responseJSON := nicovideoAPI.Parse(resp)
		log.Infof(ctx, "meta: status=%d id=%s totalCount=%d", responseJSON.Meta.Status, responseJSON.Meta.ID, responseJSON.Meta.TotalCount)
		log.Infof(ctx, "data: count=%d", len(responseJSON.Data))

		var keysToPut []*datastore.Key
		var videosToPut []Video

		for _, data := range responseJSON.Data {
			log.Debugf(ctx, "data: contentId=%s", data.ContentID)
			query := datastore.
				NewQuery("Video").
				Filter("ContentID =", data.ContentID)
			var videosFiltered []Video
			keys, err := query.GetAll(ctx, &videosFiltered)
			if err != nil {
				panic(err.Error())
			}
			log.Debugf(ctx, "count: %d", len(keys))

			var key *datastore.Key
			var video Video
			now := time.Now().
				In(location).
				Format(timeFormatISO8601)
			if len(keys) > 0 {
				key = keys[0]
				video = Video{Data: data, Tweeted: videosFiltered[0].Tweeted, LastUpdated: now}
			} else {
				key = datastore.NewIncompleteKey(ctx, "Video", nil)
				video = Video{Data: data, Tweeted: "", LastUpdated: now}
			}
			keysToPut = append(keysToPut, key)
			videosToPut = append(videosToPut, video)
		}

		if _, err := datastore.PutMulti(ctx, keysToPut, videosToPut); err != nil {
			panic(err.Error())
		}
	}

	// 結果を集計してツイートする
	{
		startTimeFrom := time.Now().
			In(location).
			AddDate(0, 0, -periodInDay).
			Format(timeFormatISO8601)
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

			if video.ViewCounter < viewCounterThreshold && video.CommentCounter < commentCounterThreshold && video.MylistCounter < mylistCounterThreshold {
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
				Format(timeFormatISO8601)
			video.Tweeted = now
			video.LastUpdated = now
			if _, err := datastore.Put(ctx, keys[index], &video); err != nil {
				panic(err.Error())
			}

			if tweetCount >= tweetLimitAtSameTime {
				break
			}

			time.Sleep(sleepDurationInSec * time.Second)
		}
	}
}
