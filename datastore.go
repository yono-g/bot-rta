package myapp

import (
	"context"
	"time"

	"google.golang.org/appengine/datastore"
)

type Video struct {
	Data
	Tweeted     string
	LastUpdated string
}

type VideoStore struct {
	kindName string
	context  context.Context
}

func NewVideoStore(context context.Context) *VideoStore {
	return &VideoStore{
		kindName: "video",
		context:  context,
	}
}

func (s *VideoStore) FindByContentID(contentID string) (*datastore.Key, *Video, error) {
	var videos []Video
	query := datastore.
		NewQuery(s.kindName).
		Filter("ContentID =", contentID)
	keys, err := query.GetAll(s.context, &videos)
	if err != nil {
		return nil, nil, err
	}

	if len(keys) < 1 {
		return nil, nil, nil
	}
	return keys[0], &videos[0], nil
}

func (s *VideoStore) FindRecent() ([]*datastore.Key, *[]Video, error) {
	location, _ := time.LoadLocation("Asia/Tokyo")
	sevenDaysAgo := time.Now().
		In(location).
		AddDate(0, 0, -7).
		Format("2006-01-02T15:04:05+09:00")
	query := datastore.
		NewQuery(s.kindName).
		Filter("StartTime >=", sevenDaysAgo).
		Order("StartTime")

	var videos []Video
	keys, err := query.GetAll(s.context, &videos)
	if err != nil {
		return nil, nil, err
	}
	return keys, &videos, nil
}

func (s *VideoStore) NewKey() *datastore.Key {
	return datastore.NewIncompleteKey(s.context, s.kindName, nil)
}

func (s *VideoStore) ExecPut(key *datastore.Key, video *Video) (*datastore.Key, error) {
	location, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().
		In(location).
		Format("2006-01-02T15:04:05+09:00")

	video.LastUpdated = now

	return datastore.Put(s.context, key, video)
}

func (s *VideoStore) ExecPutMulti(keys []*datastore.Key, videos []*Video) ([]*datastore.Key, error) {
	location, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().
		In(location).
		Format("2006-01-02T15:04:05+09:00")

	for _, video := range videos {
		video.LastUpdated = now
	}

	return datastore.PutMulti(s.context, keys, videos)
}
