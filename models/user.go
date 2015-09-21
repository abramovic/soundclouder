package models

type User struct {
	Id                   int    `json:"id"`
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
	TrackCount           int    `json:"track_count"  as:"Tracks"`
	PlaylistCount        int    `json:"playlist_count"  as:"Playlists"`
	FollowersCount       int    `json:"followers_count"  as:"Followers"`
	FollowingsCount      int    `json:"followings_count" as:"Followings"`
	PublicFavoritesCount int    `json:"public_favorites_count"  as:"Favorites"`
}
