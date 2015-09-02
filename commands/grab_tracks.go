package main

import (
	"encoding/json"
	"fmt"
	as "github.com/aerospike/aerospike-client-go"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

var (
	MAX_WORKERS int = 200
	httpClient  *http.Client
	aeroClient  *as.Client
	ASNamespace string = "test"
)

func init() {
	httpClient = createHTTPClient()
}

func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 500,
		},
		Timeout: time.Duration(120 * time.Second),
	}
	return client
}

type Track struct {
	Id           uint64 `json:"id"`
	CreatedAt    string `json:"created_at"`
	UserId       uint64 `json:"user_id"`
	Duration     uint64 `json:"duration"`
	Commentable  bool   `json:"commentable"`
	State        string `json:"state"`
	Sharing      string `json:"sharing"`
	TagList      string `json:"tag_list"`
	Permalink    string `json:"permalink"`
	Description  string `json:"description"`
	Streamable   bool   `json:"streamable"`
	Downloadable bool   `json:"downloadable"`
	Genre        string `json:"genre"`
	// Release is sometimes a string and sometimes a number... annoying
	//Release             string      `json:"release"`
	PurchaseURL         string      `json:"purchase_url"`
	LabelId             string      `json:"label_id"`
	LabelName           string      `json:"label_name"`
	ISRC                string      `json:"isrc"`
	VideoURL            string      `json:"video_url"`
	TrackType           string      `json:"track_type"`
	KeySignature        string      `json:"key_signature"  as:"Signature"`
	BPM                 float64     `json:"bpm"`
	Title               string      `json:"title"`
	ReleaseYear         uint        `json:"release_year"  as:"RYear"`
	ReleaseMonth        uint        `json:"release_month"  as:"RMonth"`
	ReleaseDay          uint        `json:"release_day"  as:"RDay"`
	OriginalFormat      string      `json:"original_format" as:"OFormat"`
	OriginalContentSize uint64      `json:"original_content_size" as:"OSize"`
	License             string      `json:"license"`
	URI                 string      `json:"uri"`
	PermalinkURL        string      `json:"permalink_url"`
	ArtworkURL          string      `json:"artwork_url"`
	WaveformURL         string      `json:"waveform_url"`
	User                UserPreview `json:user`
	StreamURL           string      `json:"stream_url"`
	DownloadURL         string      `json:"download_url"`
	PlaybackCount       uint64      `json:"playback_count"  as:"Playbacks"`
	DownloadCount       uint64      `json:"download_count"  as:"Downloads"`
	FavoritingsCount    uint64      `json:"favoritings_count" as:"Favorites"`
	CommentCount        uint64      `json:"comment_count"  as:"Comments"`
	CreatedWith         CreatedWith `json:"created_with"  as:"CreatedW"`
	AttachmentsURI      string      `json:"attachments_uri"  as:"Attachments"`
	IgnoreCrawl         int         `json:"-"`
	FirstCrawl          int64       `json:"-"`
	LastCrawl           int64       `json:"-"`
}

type UserPreview struct {
	Id           uint64 `json:"id"`
	Permalink    string `json:"permalink"`
	URI          string `json:"uri"`
	PermalinkURL string `json:"permalink_url"`
	AvatarURL    string `json:"avatar_url"`
	FirstCrawl   int64  `json:"-"`
	LastCrawl    int64  `json:"-"`
}
type CreatedWith struct {
	Id           uint64 `json:"id"`
	Name         string `json:"name"`
	URI          string `json:"uri"`
	PermalinkURL string `json:"permalink_url"`
}

type User struct {
	Id                   uint64 `json:"id"`
	Permalink            string `json:"permalink"`
	URI                  string `json:"uri"`
	PermalinkURL         string `json:"permalink_url"`
	AvatarURL            string `json:"avatar_url"`
	Username             string `json:"username"`
	Country              string `json:"country"`
	Fullname             string `json:"full_name"`
	City                 string `json:"city"`
	Destination          string `json:"description"`
	DiscogsName          string `json:"discogs_name"`
	MySpaceName          string `json:"myspace_name"`
	Website              string `json:"website"`
	WebsiteTitle         string `json:"website_title"  as:"WebTitle"`
	Online               bool   `json:"online"`
	TrackCount           uint64 `json:"track_count"  as:"Tracks"`
	PlaylistCount        uint64 `json:"playlist_count"  as:"Playlists"`
	FollowersCount       uint64 `json:"followers_count"  as:"Followers"`
	FollowingsCount      uint64 `json:"followings_count" as:"Followings"`
	PublicFavoritesCount uint64 `json:"public_favorites_count"  as:"Favorites"`
}

var (
	track_ids chan int
	crawled   int
	found     int
	start     time.Time
)

type Configuration struct {
	Host       string `json:"host"`
	ClientId   string `json:"client_id"`
	MaxWorkers int    `json:"max_workers"`
	Namespace  string `json:"namespace"`
}

func main() {
	start = time.Now()
	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	decoder := json.NewDecoder(file)
	config := Configuration{}
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if config.Host == "" || config.ClientId == "" {
		fmt.Println("Missing Configs", config)
		os.Exit(1)
	}

	if config.Namespace == "" {
		config.Namespace = "test"
	}
	ASNamespace = config.Namespace

	if config.MaxWorkers > 0 {
		MAX_WORKERS = config.MaxWorkers
	}

	aeroClient, err = as.NewClient(config.Host, 3000)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if aeroClient.IsConnected() == false {
		fmt.Println("Not connected to Aerospike")
		os.Exit(1)
	}

	{
		idxTask, _ := aeroClient.CreateIndex(nil, ASNamespace, "track", "trackUserId", "UserId", as.NUMERIC)
		if idxTask != nil {
			<-idxTask.OnComplete()
		}
	}

	{
		idxTask, _ := aeroClient.CreateIndex(nil, ASNamespace, "track", "trackFirstCrawl", "FirstCrawl", as.NUMERIC)
		if idxTask != nil {
			<-idxTask.OnComplete()
		}
	}

	{
		idxTask, _ := aeroClient.CreateIndex(nil, ASNamespace, "track", "trackLastCrawl", "LastCrawl", as.NUMERIC)
		if idxTask != nil {
			<-idxTask.OnComplete()
		}
	}

	crawler := &Crawler{config.ClientId}
	max_id, err := crawler.GetHighTrackId()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	var trackMonitor sync.WaitGroup
	track_ids = make(chan int, 2000000)
	for i := 0; i < MAX_WORKERS; i++ {
		go crawler.ProcessTracks(&trackMonitor)
	}

	for i := max_id; i > 0; i-- {
		val := fmt.Sprintf("%d", i)
		if len(os.Args) > 2 {
			if os.Args[2] == val[len(val)-1:] {
				track_ids <- i
			}
		} else {
			track_ids <- i
		}
	}
	close(track_ids)
	trackMonitor.Wait()
}

func (c *Crawler) ProcessTracks(wg *sync.WaitGroup) error {
T:
	for {
		select {
		case track_id, open := <-track_ids:
			if !open {
				break T
			}
			var err error
			now := time.Now().UTC().Unix()
			key, _ := as.NewKey(ASNamespace, "track", track_id)
			var t Track
			aeroClient.GetObject(nil, key, &t)
			if t.IgnoreCrawl == 1 {
				// we can't crawl this track
				continue
			}
			if (crawled % 1000) == 0 {
				fmt.Printf("Crawled\t%d\tFound\t%d\t%s\t%d\n", crawled, found, time.Since(start), track_id)
			}
			crawled++
			track, err := c.GetTrack(track_id)
			if err != nil {
				t.Id = uint64(track_id)
				t.FirstCrawl = now
				t.LastCrawl = now
				t.IgnoreCrawl = 1
				aeroClient.PutObject(nil, key, &t)
				continue
			}
			found++
			if track.FirstCrawl == 0 {
				track.FirstCrawl = now
			}
			track.LastCrawl = now
			aeroClient.PutObject(nil, key, track)
			keyTS, _ := as.NewKey(ASNamespace, "track_crawl", fmt.Sprintf("%d-%d", now, track_id))
			aeroClient.PutObject(nil, keyTS, track)
		}
	}
	wg.Done()
	return nil
}

type Crawler struct {
	ClientId string
}

func (c *Crawler) GetHighTrackId() (int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/tracks?client_id=%s&limit=1&created_at[from]=", c.ClientId), nil)
	if err != nil {
		return 0, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var t []Track
	err = json.Unmarshal(body, &t)
	if err != nil {
		return 0, err
	}

	return int(t[0].Id), nil
}

func (c *Crawler) GetTrack(id int) (*Track, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://api.soundcloud.com/tracks/%d?client_id=%s", id, c.ClientId), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
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

	var t Track
	err = json.Unmarshal(body, &t)
	if err != nil {
		fmt.Println(string(body))
		return nil, err
	}
	return &t, nil
}
