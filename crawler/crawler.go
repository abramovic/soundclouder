package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/Abramovic/soundclouder/config"
	"github.com/Abramovic/soundclouder/models"
	"github.com/carlescere/goback"
	"github.com/garyburd/redigo/redis"
	"io/ioutil"
	"net/http"
	"time"
)

// A Crawler that contains a Client ID and a pool of Redis connections and one shared HTTP connection
type Crawler struct {
	ClientId    string
	RedisClient *redis.Pool
	HttpClient  *http.Client
	BackOff     *goback.SimpleBackoff
}

var domain string = "http://api.soundcloud.com"

func New(config config.Configuration) *Crawler {
	return &Crawler{
		ClientId:    config.ClientId,
		HttpClient:  CreateHTTPClient(),
		RedisClient: CreateRedisClient(config.Host, 6379),
		BackOff:     CreateGoback(),
	}
}

func (c *Crawler) Close() error {
	return c.RedisClient.Close()
}

func CreateGoback() *goback.SimpleBackoff {
	return &goback.SimpleBackoff{
		Min:    5 * time.Second,
		Max:    2 * time.Hour,
		Factor: 2,
	}
}

func CreateRedisClient(host string, port int) *redis.Pool {
	// Creates a pointer to a pool of Redis connections
	return &redis.Pool{
		MaxIdle:     0,
		MaxActive:   1000, // We might want to change this to improve performance.
		IdleTimeout: 1 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func CreateHTTPClient() *http.Client {
	// This creates a reusable http client instead of creating a new client with each request.
	// This is more efficient for what we are trying to accomplish.
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 500,
		},
		Timeout: time.Duration(120 * time.Second),
	}
	return client
}

// We will use Redis Hashes for a more memory efficent way of storing data.
func RedisKey(prefix string, id int) (string, string) {
	i := (id / 1000)
	return fmt.Sprintf("%s:%d", prefix, i), fmt.Sprintf("%d", id)
}

func (c *Crawler) Wait() {
	if c.BackOff != nil {
		goback.Wait(c.BackOff)
		return
	}
	time.Sleep(5 * time.Second)
}

// The /tracks endpoint without a track_id searches for track using the query paramaters.
// Normally you would want to use "q=search-term" but we want to look at all of the songs
// By including the created_at query param we will get back the most recent tracks on SoundCloud
// Limit the results to just one track since we just need the highest track id
func (c *Crawler) GetHighTrackId() (int, error) {
	url := fmt.Sprintf("%s/tracks?client_id=%s&limit=1&created_at[from]=", domain, c.ClientId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var t []models.Track
	err = json.Unmarshal(body, &t)
	if err != nil {
		return 0, err
	}
	// Since we are getting back an array from SoundCloud we only want to return the first element
	return t[0].Id, nil
}

// Getting a SoundCloud playlist using an HTTP Client instead of the SoundCloud Go library.
func (c *Crawler) GetPlaylist(id int) (*models.Playlist, error) {
	var err error
	var p models.Playlist
	url := fmt.Sprintf("%s/playlists/%d?client_id=%s", domain, id, c.ClientId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		// We most likely hit some issue with SoundCloud... time to back off
		c.Wait()
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Get a track using the standard http client
func (c *Crawler) GetTrack(id int) (*models.Track, error) {
	var t models.Track
	url := fmt.Sprintf("%s/tracks/%d?client_id=%s", domain, id, c.ClientId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		// We most likely hit some issue with SoundCloud... time to back off
		c.Wait()
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (c *Crawler) getTrackFavoriters(id, offset int) []models.Favoriter {
	var favoriters []models.Favoriter
	url := fmt.Sprintf("%s/tracks/%d/favoriters?client_id=%s&limit=200&offset=%d", domain, id, c.ClientId, offset)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return favoriters
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		// We most likely hit some issue with SoundCloud... time to back off
		c.Wait()
		return favoriters
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return favoriters
	}
	err = json.Unmarshal(body, &favoriters)
	if err != nil {
		return favoriters
	}
	return favoriters
}

// Get all of the favoriters from a Track. We can get up to 200 results at a time and use an offset to grab all of the favoriters
func (c *Crawler) GetTrackFavoriters(id int) []models.Favoriter {
	var offset int = 0
	var favoriters []models.Favoriter
	for {
		results := c.getTrackFavoriters(id, offset)
		for _, result := range results {
			favoriters = append(favoriters, result)
		}
		if len(results) < 190 {
			// in case we don't get all of the 200 results...
			// Sometimes SoundCloud is funky and returns 199 results instead of 200 (even if there are more remaining)
			break
		}
		offset += 200
	}
	return favoriters
}

func (c *Crawler) getTrackComments(id, offset int) []models.Comment {
	var comments []models.Comment
	url := fmt.Sprintf("%s/tracks/%d/comments?client_id=%s&limit=200&offset=%d", domain, id, c.ClientId, offset)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return comments
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		// We most likely hit some issue with SoundCloud... time to back off
		c.Wait()
		return comments
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return comments
	}
	err = json.Unmarshal(body, &comments)
	if err != nil {
		return comments
	}
	return comments
}

func (c *Crawler) GetTrackComments(id int) []models.Comment {
	var offset int = 0
	var comments []models.Comment
	for {
		results := c.getTrackComments(id, offset)
		for _, result := range results {
			comments = append(comments, result)
		}
		if len(results) < 190 {
			// in case we don't get all of the 200 results...
			// Sometimes SoundCloud is funky and returns 199 results instead of 200 (even if there are more remaining)
			break
		}
		// Increment the offset by 200 even if we don't get back 200 results.
		offset += 200
	}
	return comments
}
