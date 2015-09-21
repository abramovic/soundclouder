package models

type Playlist struct {
	Id   int         `json:"id"`
	User UserPreview `json:"user"`
	// SoundCloud is not very consistent with the Track object.
	// When getting all of the tracks by a playlist some of the fields switch from ints to strings
	Tracks []TrackIds `json:"tracks"`
}

type TrackIds struct {
	Id     int `json:"id"`
	UserId int `json:"user_id"`
}
