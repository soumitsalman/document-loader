package document

import (
	"encoding/json"
	"fmt"

	datautils "github.com/soumitsalman/data-utils"
)

type Document struct {
	URL         string `json:"url,omitempty"`
	Source      string `json:"source,omitempty"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	Author      string `json:"author,omitempty"`
	PublishDate int64  `json:"created,omitempty"`
	LoadDate    int64  `json:"loaded,omitempty"`
	Category    string `json:"category,omitempty"`
	Comments    int    `json:"comments,omitempty"`
	Likes       int    `json:"likes,omitempty"`
}

func (c *Document) ToString() string {
	// TODO: remove. temp for debugging
	c.Body = datautils.TruncateTextWithEllipsis(c.Body, 150)
	json_data, _ := json.MarshalIndent(c, "", "\t")
	return fmt.Sprint(string(json_data))
}
