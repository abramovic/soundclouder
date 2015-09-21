package models

type Comment struct {
	Id        int    `json:"id"`
	Kind      string `json:"kind"`
	UserId    int    `json:"user_id"`
	TrackId   int    `json:"track_id"`
	Timestamp int    `json:"timestamp"`
	Body      string `json:"body"`
	User      User   `json:"user"`
}
