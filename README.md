## soundclouder
#### Crawl the SoundCloud graph at scale. 

This is a simple crawler written in Go (Golang) that grabs all of the edges in the **SoundCloud graph**. An edge consists of a playlist, comment and favorite of a track. 

### Design Goals

+ **Crawl Everything**: Be able to grab all of the artists (users with tracks), tracks, comments, favorites and playlists on SoundCloud. 
+ **Multiple Workers**: It is semi-distributed where multiple workers can connect to Redis and start crawling SoundCloud. Originally I used Aerospike but since Redis is more popular I wanted to go with something that other people are already familiar with. 
+ **Failover Support**: If a worker dies, you can retry the failed crawls. This is done manually for now via command line flags. 
+ **Graceful Exits**: Support for graceful exits of the program. If you give an interrupt signal, then the program will finish up existing crawls and not start any new ones. 
+ **No Crazy Dependencies**: I tried to make this project really simple. There are dependencies outside of using [Redis](http://redis.io) for the database. The only third-party library that I included is [Redigo](https://github.com/garyburd/redigo) by Gary Burd for connecting to Redis. 
+ **Multi-Core Processing**: This program utilizes multiple CPU cores so don't be afraid to use one big large box when running this! 
+ **Parallel Crawling**: Okay, I know Rob Pike said that Go's [Concurrency is not Parallelism](https://talks.golang.org/2012/waza.slide#1) but we do spawn multiple crawlers via goroutines which will speed up your crawl times significantly. 

### How to Run

It should be pretty easy to get started. 

+ **Build**: go build .
+ **Run**: ./soundclouder -config="/path/to/your/config.json" 

A sample configuration file is already provided for you. Just fill in your client_id and the hostname of your Redis database. 

### TODO

+ Export from Redis to GraphLab so you can use Dato/GraphLab to process your crawls. 
+ Constant crawling and timestamps of crawls so we can see how the edges in the SoundCloud graph change over time.

### FAQ

**Can I add more workers?**
Yes you can! By default I am only using 200 but you can change the number of workers (goroutines) from your configuration file.

**Why only one Client ID?**
There are no limits to crawling public tracks and playlists according to the SoundCloud API. 

**What's with the Redis hashes?**
Instead of using sets or lists in Redis, I chose to use hashes. The reason why is because storing data as hash keys is more memory efficient than storing as regular keys. Everything is saved as either JSON or a comma delimited string. 

**How is this distributed?**
When the program is run you can tell it to do a blank slate crawl (configured by default) or throw in a "empty" flag to just process any pending crawls. "./soundclouder -empty=false"

The program will empty out as many crawls as possible from Redis and then start processing it. If you have multiple workers connecting with "empty=false" then they will just take the next list of crawls from Redis. Each crawl contains up to 1,000 children crawls (because of how we are storing hashes in Redis). 

**What's with the todo sets?**
There are four primary sets that handle crawls. There is a master playlist and track crawler set and a pending/incomplete set for the tracks and playlists. If a worker dies we can restart all of the incomplete jobs by using a command line flag "./soundclouder -restart=true" that will add those crawls back to the list of master crawls. 

**How do you handle non-existent/public tracks?**
When the crawler first runs it assumes every track and playlist need to be crawled. We store a "null" value for any pending crawl and if the result turns up empty then we delete the value from Redis and will not make a request to the SoundCloud API the second time around. 

**How do you handle reaching the largest playlist/track/etc?**
Using SoundCloud's search endpoint we can get the most recent track made and we assume that is the max track id. There's no way of doing this with playlists so we rely on you to provide the max id as a command line flag (we default to 30,000,000). 

There are ways to guestimate the max playlist id that might be released in a later version of this. 

**Do you crawl all of the users on SoundCloud?**
Currently this is configured to only store information about users who have created content on SoundCloud. This includes users who have favorited or commented on a track. 

**Why do you not store all of the information about a track or user?**
We might not store all of the information about a track due to inconsistencies with the SoundCloud API. If there is something missing about the user then it's probably either due to other API inconsistencies or at the time of building this tool I didn't see a need for storing everything about the user. 

The top priority with this tool is the abilityto crawl as many edges of the SoundCloud graph (favorite/comment/playlist) as possible and not the metadata on the nodes. If more information is needed about the nodes we always have the IDs in Redis to crawl again later. 
