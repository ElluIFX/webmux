package session

import "encoding/json"

func (e Event) MarshalJSON() ([]byte, error) {
	if e.Type == "snapshot" {
		return json.Marshal(struct {
			Type     string `json:"type"`
			Sessions []Info `json:"sessions"`
		}{e.Type, e.Sessions})
	}
	type plain Event
	return json.Marshal(plain(e))
}
