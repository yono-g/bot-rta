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

const (
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

	twitterAPI := anaconda.NewTwitterApiWithCredentials(
		os.Getenv("TWITTER_ACCESS_TOKEN"),
		os.Getenv("TWITTER_ACCESS_TOKEN_SECRET"),
		os.Getenv("TWITTER_CONSUMER_KEY"),
		os.Getenv("TWITTER_CONSUMER_SECRET"),
	)
	twitterAPI.HttpClient.Transport = &urlfetch.Transport{Context: ctx}

	videoStore := NewVideoStore(ctx)

	// niconico コンテンツ検索APIを叩いてデータを集める
	{
		nicovideoAPI := NewNicovideoAPIClient(
			os.Getenv("APP_NAME"),
			os.Getenv("USER_AGENT"),
		)
		nicovideoAPI.HTTPClient.Transport = &urlfetch.Transport{Context: ctx}

		var offset int
		for {
			req, resp := nicovideoAPI.Get(offset)
			log.Infof(ctx, "request url: %s", req.URL)
			log.Infof(ctx, "response status: %s", resp.Status)

			responseJSON := nicovideoAPI.Parse(resp)
			videoCount := len(responseJSON.Data)
			log.Infof(ctx, "meta: status=%d id=%s totalCount=%d", responseJSON.Meta.Status, responseJSON.Meta.ID, responseJSON.Meta.TotalCount)
			log.Infof(ctx, "data: count=%d", videoCount)

			var keysToPut []*datastore.Key
			var videosToPut []*Video
			for _, data := range responseJSON.Data {
				log.Debugf(ctx, "data: contentId=%s", data.ContentID)
				key, video, err := videoStore.FindOrNew(data.ContentID)
				if err != nil {
					panic(err.Error())
				}
				video.Data = data

				keysToPut = append(keysToPut, key)
				videosToPut = append(videosToPut, video)
			}
			if _, err := videoStore.ExecPutMulti(keysToPut, videosToPut); err != nil {
				panic(err.Error())
			}

			offset += videoCount
			if offset >= responseJSON.Meta.TotalCount {
				break
			}
		}
	}

	// 結果を集計してツイートする
	{
		keys, videos, err := videoStore.FindRecent()
		if err != nil {
			panic(err.Error())
		}
		log.Debugf(ctx, "query result: count=%d", len(keys))

		location, _ := time.LoadLocation("Asia/Tokyo")
		tweetCount := 0
		for index, video := range *videos {
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
				Format("2006-01-02T15:04:05+09:00")
			video.Tweeted = now
			if _, err := videoStore.ExecPut(keys[index], &video); err != nil {
				panic(err.Error())
			}

			if tweetCount >= tweetLimitAtSameTime {
				break
			}

			time.Sleep(sleepDurationInSec * time.Second)
		}
	}
}
