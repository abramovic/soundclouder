package crawler

import (
	"encoding/json"
	"fmt"
	"github.com/Abramovic/soundclouder/models"
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

// The /tracks endpoint without a track_id searches for track using the query paramaters.
// Normally you would want to use "q=search-term" but we want to look at all of the songs
// By including the created_at query param we will get back the most recent tracks on SoundCloud
// Limit the results to just one track since we just need the highest track id
func (c *Crawler) GetHighTrackId() (int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/tracks?client_id=%s&limit=1&created_at[from]=", c.ClientId), nil)
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
	req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/playlists/%d?client_id=%s", id, c.ClientId), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		// We most likely hit some issue with SoundCloud... time to back off
		time.Sleep(5 * time.Second)
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var p models.Playlist
	err = json.Unmarshal(body, &p)
	if err != nil {
		fmt.Println("Parse Playlist", err)
		return nil, err
	}
	return &p, nil
}

// Get a track using the standard http client
func (c *Crawler) GetTrack(id int) (*models.Track, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/tracks/%d?client_id=%s", id, c.ClientId), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		// We most likely hit some issue with SoundCloud... time to back off
		time.Sleep(5 * time.Second)
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var t models.Track
	err = json.Unmarshal(body, &t)
	if err != nil {
		fmt.Println("Parse Track", err)
		return nil, err
	}
	return &t, nil
}

// Get all of the favoriters from a Track. We can get up to 200 results at a time and use an offset to grab all of the favoriters
func (c *Crawler) GetTrackFavoriters(id int) []models.Favoriter {
	var offset int = 0
	var favoriters []models.Favoriter
	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/tracks/%d/favoriters?client_id=%s&limit=200&offset=%d", id, c.ClientId, offset), nil)
		if err != nil {
			break
		}
		resp, err := c.HttpClient.Do(req)
		if err != nil {
			// We most likely hit some issue with SoundCloud... time to back off
			time.Sleep(5 * time.Second)
			break
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			break
		}
		var results []models.Favoriter
		err = json.Unmarshal(body, &results)
		if err != nil {
			break
		}
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

func (c *Crawler) GetTrackComments(id int) []models.Comment {
	var offset int = 0
	var comments []models.Comment
	for {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/tracks/%d/comments?client_id=%s&limit=200&offset=%d", id, c.ClientId, offset), nil)
		if err != nil {
			break
		}
		resp, err := c.HttpClient.Do(req)
		if err != nil {
			// We most likely hit some issue with SoundCloud... time to back off
			time.Sleep(5 * time.Second)
			break
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			break
		}
		var results []models.Comment
		err = json.Unmarshal(body, &results)
		if err != nil {
			break
		}
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
