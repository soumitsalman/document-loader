package document

import (
	"encoding/json"
	"fmt"

	datautils "github.com/soumitsalman/data-utils"
)

type Document struct {
	Kind        string   `json:"kind,omitempty"`
	URL         string   `json:"url,omitempty"`
	Source      string   `json:"source,omitempty"`
	Title       string   `json:"title,omitempty"`
	Text        string   `json:"text,omitempty"`
	Author      string   `json:"author,omitempty"`
	PublishDate int64    `json:"created,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
	Comments    int      `json:"comments,omitempty"`
	Likes       int      `json:"likes,omitempty"`
}

func (c *Document) String() string {
	// TODO: remove. temp for debugging
	c.Text = datautils.TruncateTextWithEllipsis(c.Text, 150)
	json_data, _ := json.MarshalIndent(c, "", "\t")
	return fmt.Sprint(string(json_data))
}
