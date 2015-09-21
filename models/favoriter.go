package models

type Favoriter struct {
	Id                   int    `json:"id"`
	Kind                 string `json:"kind"`
	TrackCount           int    `json:"track_count"`
	PlaylistCount        int    `json:"playlist_count"`
	PublicFavoritesCount int    `json:"public_favorites_count"`
	FollowersCount       int    `json:"followers_count"`
	FollowingsCount      int    `json:"followings_count"`
}
