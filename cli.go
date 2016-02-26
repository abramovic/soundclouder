package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Abramovic/soundclouder/config"
	"github.com/Abramovic/soundclouder/crawler"
	"github.com/Abramovic/soundclouder/helpers"
	"github.com/garyburd/redigo/redis"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var (
	max_workers  int = 200
	playlist_ids chan int
	track_ids    chan int
	maxPlaylist  = flag.Int("playlist", 40000000, "max playlist id") // We can't automatically grab the max playlist id
	useEmpty     = flag.Bool("empty", true, "use empty database")
	configFile   = flag.String("config", "", "path to config file")
	restartTodo  = flag.Bool("restart", false, "restart incomplete crawls due to a crash")
)

/*

	NOTES:

	We can probably merge the two channels (playlists and tracks) into one batch crawl channel
	type BatchCrawl struct {
		BatchId int
		Type string // track or playlist
	}

*/

type Crawler struct {
	*crawler.Crawler
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error

	file, err := os.Open(*configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	decoder := json.NewDecoder(file)
	config := config.Configuration{}
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if config.Host == "" || config.ClientId == "" {
		fmt.Println("Missing Configs", config)
		os.Exit(1)
	}

	if config.MaxWorkers > 0 {
		max_workers = config.MaxWorkers
	}

	c := crawler.New(config)
	defer c.Close()

	crawler := Crawler{c}

	// We are able to get the highest track id on our own.
	max_id, err := crawler.GetHighTrackId()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Handle signals to stop crawling.
	var canCrawl bool = true
	stopCrawler := make(chan os.Signal, 1)
	signal.Notify(stopCrawler, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopCrawler
		fmt.Println("Exit signal recieved. Will finish up current crawls and exit the program.")
		canCrawl = false
		close(track_ids)
		close(playlist_ids)
	}()

	track_ids = make(chan int, max_workers)
	var trackMonitor sync.WaitGroup
	trackMonitor.Add(max_workers)
	for i := 0; i < max_workers; i++ {
		go crawler.ProcessTracks(&trackMonitor)
	}

	r := c.RedisClient.Get()
	defer r.Close()

	// If we want to restart crawls due to a server crash...
	if *restartTodo == true && *useEmpty == false && canCrawl {
		playlists, _ := redis.Ints(r.Do("SMEMBERS", "crawlPlaylistsTodo"))
		for _, i := range playlists {
			r.Do("SADD", "crawlPlaylists", i)
		}
		tracks, _ := redis.Ints(r.Do("SMEMBERS", "crawlPlaylistsTodo"))
		for _, i := range tracks {
			r.Do("SADD", "crawlTracks", i)
		}
	}

	// Starts a new crawl from scratch...
	if *useEmpty == true && canCrawl {
		r.Do("DEL", "crawlTracks")
		batch_max := int(max_id/1000) + 1
		for i := batch_max; i > 0; i-- {
			nulls := []interface{}{fmt.Sprintf("trackMeta:%d", i)}
			track_id := i * 1000
			for k := 999; k >= 0; k-- {
				nulls = append(nulls, fmt.Sprintf("%d", (track_id+k)), "null")
			}
			r.Do("HMSET", nulls...)
			r.Do("SADD", "crawlTracks", i)
		}
	}

	var hasMoreTracks bool = true
	for hasMoreTracks && canCrawl {
		// Add all of the tracks that are scheduled to be crawled into a channel
		i, err := redis.Int(r.Do("SPOP", "crawlTracks"))
		if err != nil {
			hasMoreTracks = false
		} else {
			r.Do("SADD", "crawlTracksTodo", i)
			track_ids <- i
		}
	}
	close(track_ids)
	// Wait for all of the tracks to be crawled before going onto the playlists
	trackMonitor.Wait()

	// Manually run garbage collection to free up any memory that is no longer used
	runtime.GC()

	playlist_ids = make(chan int, max_workers)
	var playlistMonitor sync.WaitGroup
	playlistMonitor.Add(max_workers)
	for i := 0; i < max_workers; i++ {
		go crawler.ProcessPlaylists(&playlistMonitor)
	}
	if *useEmpty == true && canCrawl {
		r.Do("DEL", "crawlPlaylists")
		playlist_max := int(*maxPlaylist/1000) + 1
		for i := playlist_max; i > 0; i-- {
			nulls := []interface{}{fmt.Sprintf("playlistTracks:%d", i)}
			playlist_id := i * 1000
			for k := 999; k >= 0; k-- {
				nulls = append(nulls, fmt.Sprintf("%d", (playlist_id+k)), "null")
			}
			r.Do("HMSET", nulls...)
			r.Do("SADD", "crawlPlaylists", i)
		}
	}

	var hasMorePlaylists bool = true
	for hasMorePlaylists && canCrawl {
		i, err := redis.Int(r.Do("SPOP", "crawlPlaylists"))
		if err != nil {
			hasMorePlaylists = false
		} else {
			r.Do("SADD", "crawlPlaylistsTodo", i)
			playlist_ids <- i
		}
	}
	close(playlist_ids)
	playlistMonitor.Wait()
}

func (c *Crawler) ProcessPlaylists(wg *sync.WaitGroup) error {
	r := c.RedisClient.Get()
	defer r.Close()
P:
	for {
		select {
		case batch_id, open := <-playlist_ids:
			if !open {
				break P
			}
			// Get all of the IDs in this batch of 1,000 playlists
			ids, err := redis.Strings(r.Do("HKEYS", fmt.Sprintf("playlistTracks:%d", batch_id)))
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, id := range ids {
				// CLEANUP Go's string to int is string to int64 and then we are turning the int64 into an int
				pid, err := strconv.ParseInt(id, 0, 64)
				if err != nil {
					continue
				}
				playlist_id := int(pid)
				key, hkey := crawler.RedisKey("playlistTracks", playlist_id)
				exists, _ := redis.Bool(r.Do("HEXISTS", key, hkey))
				if exists == false {
					// we can't crawl this record because it was deleted before in a past crawl.
					continue
				}
				playlist, err := c.GetPlaylist(playlist_id)
				if err != nil {
					// We can't crawl this record. Make sure we delete it from our database.
					r.Do("HDEL", key, hkey)
					continue
				}
				track_ids := []string{}
				for _, track := range playlist.Tracks {
					// AppendSlice keeps a unique slice in case the playlist has the same track multiple times
					track_ids = helpers.AppendSlice(track_ids, fmt.Sprintf("%d", track.Id))
				}
				if len(track_ids) == 0 {
					// This playlist doesn't have any tracks associated with it
					r.Do("HDEL", key, hkey)
					continue
				}
				// If this is the first time that we have seen this playlist (empty string or "null") then we
				// want to increment the counter for each track in the playlist
				s, _ := redis.String(r.Do("HGET", key, hkey))
				if s == "null" || s == "" {
					for _, track := range playlist.Tracks {
						// Increment the counter for the tracks, not the playlist
						k, h := crawler.RedisKey("trackCountPlaylist", track.Id)
						r.Do("HINCRBY", k, h, 1)
					}
				}
				r.Do("HSET", key, hkey, strings.Join(track_ids, ","))
			}
			r.Do("SREM", "crawlPlaylistsTodo", batch_id)
		}
	}
	wg.Done()
	return nil
}

func (c *Crawler) ProcessTracks(wg *sync.WaitGroup) error {
	r := c.RedisClient.Get()
	defer r.Close()
T:
	for {
		select {
		case batch_id, open := <-track_ids:
			if !open {
				break T
			}
			// Grab the batch of tracks to be crawled (up to 1,000)
			ids, err := redis.Strings(r.Do("HKEYS", fmt.Sprintf("trackMeta:%d", batch_id)))
			if err != nil {
				fmt.Println(err)
				continue
			}
			for _, id := range ids {
				tid, err := strconv.ParseInt(id, 0, 64)
				if err != nil {
					continue
				}
				track_id := int(tid)

				key, hkey := crawler.RedisKey("trackMeta", track_id)
				exists, _ := redis.Bool(r.Do("HEXISTS", key, hkey))
				if exists == false {
					// we can't crawl this record because it was deleted before in a past crawl.
					continue
				}
				track, err := c.GetTrack(track_id)
				if err != nil {
					// We can't crawl this record. Make sure we delete it from our database.
					r.Do("HDEL", key, hkey)
					continue
				}
				j, err := json.Marshal(track)
				if err != nil {
					r.Do("HDEL", key, hkey)
					continue
				}
				r.Do("HSET", key, hkey, string(j))
				if track.User.Id > 0 {
					// Store the user meta data if available
					userKey, userHkey := crawler.RedisKey("userMeta", track.User.Id)
					s, _ := redis.String(r.Do("HGET", userKey, userHkey))
					if s == "null" || s == "" {
						// Only update the user data if this is the first time that we have seen them
						j, err := json.Marshal(track.User)
						if err != nil {
							continue
						}
						r.Do("HSET", userKey, userHkey, string(j))
					}
				}

				// Get all of the comments on this track
				comments := c.GetTrackComments(track.Id)
				// Transform all of the User Ids from ints into strings
				track_commenters := []string{}
				for _, comment := range comments {
					// AppendSlice will only append to the slice if the user id does not already exist
					track_commenters = helpers.AppendSlice(track_commenters, fmt.Sprintf("%d", comment.UserId))
				}
				if len(track_commenters) > 0 {
					comKey, comHkey := crawler.RedisKey("trackCommenters", track_id)
					// If this is the first time that we have seen this track (empty string or "null") then we
					// want to increment the counter of the number of user commenters that we have seen
					s, _ := redis.String(r.Do("HGET", comKey, comHkey))
					if s == "null" || s == "" {
						for range track_commenters {
							k, h := crawler.RedisKey("trackCountCommenters", track_id)
							r.Do("HINCRBY", k, h, 1)
						}
					}
					r.Do("HSET", comKey, comHkey, strings.Join(track_commenters, ","))
				}
				// Get all of the users who have favorited this track
				favoriters := c.GetTrackFavoriters(track.Id)
				// Transform the User IDs from a slice of ints to a slice of strings
				track_favoriters := []string{}
				for _, favorite := range favoriters {
					track_favoriters = append(track_favoriters, fmt.Sprintf("%d", favorite.Id))
				}
				if len(track_favoriters) > 0 {
					favKey, favHkey := crawler.RedisKey("trackFavoriters", track_id)
					// If this is the first time that we have seen this track (empty string or "null") then we
					// want to increment the counter of the number of user favorites that we have seen
					s, _ := redis.String(r.Do("HGET", favKey, favHkey))
					if s == "null" || s == "" {
						for range track_favoriters {
							k, h := crawler.RedisKey("trackCountFavoriters", track_id)
							r.Do("HINCRBY", k, h, 1)
						}
					}
					r.Do("HSET", favKey, favHkey, strings.Join(track_favoriters, ","))
				}
			}
			r.Do("SREM", "crawlTracksTodo", batch_id)
		}
	}
	wg.Done()
	return nil
}
