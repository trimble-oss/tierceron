package util

//	type ChatPost struct {
//		Type      string  `json:"type"`
//		EventTime string  `json:"eventTime"`
//		Message   Message `json:"message"`
//		User      User    `json:"user"`
//		Space     Space   `json:"space"`
//	}
type Message struct {
	Text string `json:"text"`
}
type User struct {
	DisplayName string `json:"displayName"`
}
type Space struct {
	Name string `json:"name"`
}
