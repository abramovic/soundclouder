package models

type Track struct {
	Id           int    `json:"id"`
	CreatedAt    string `json:"created_at"`
	UserId       int    `json:"user_id"`
	Duration     int    `json:"duration"`
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
	BPM                 float32     `json:"bpm"`
	Title               string      `json:"title"`
	ReleaseYear         uint        `json:"release_year"  as:"RYear"`
	ReleaseMonth        uint        `json:"release_month"  as:"RMonth"`
	ReleaseDay          uint        `json:"release_day"  as:"RDay"`
	OriginalFormat      string      `json:"original_format" as:"OFormat"`
	OriginalContentSize int         `json:"original_content_size" as:"OSize"`
	License             string      `json:"license"`
	URI                 string      `json:"uri"`
	PermalinkURL        string      `json:"permalink_url"`
	ArtworkURL          string      `json:"artwork_url"`
	WaveformURL         string      `json:"waveform_url"`
	User                UserPreview `json:"user"`
	StreamURL           string      `json:"stream_url"`
	DownloadURL         string      `json:"download_url"`
	PlaybackCount       int         `json:"playback_count"  as:"Playbacks"`
	DownloadCount       int         `json:"download_count"  as:"Downloads"`
	FavoritingsCount    int         `json:"favoritings_count" as:"Favorites"`
	CommentCount        int         `json:"comment_count"  as:"Comments"`
	CreatedWith         CreatedWith `json:"created_with"  as:"CreatedW"`
	AttachmentsURI      string      `json:"attachments_uri"  as:"Attachments"`
	IgnoreCrawl         int         `json:"-"`
	FirstCrawl          int64       `json:"-"`
	LastCrawl           int64       `json:"-"`
}

type UserPreview struct {
	Id           int    `json:"id"`
	Permalink    string `json:"permalink"`
	URI          string `json:"uri"`
	PermalinkURL string `json:"permalink_url"`
	AvatarURL    string `json:"avatar_url"`
	FirstCrawl   int    `json:"-"`
	LastCrawl    int    `json:"-"`
}
type CreatedWith struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	URI          string `json:"uri"`
	PermalinkURL string `json:"permalink_url"`
}
