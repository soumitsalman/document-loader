package collectors

import (
	"encoding/json"
	"fmt"

	"github.com/gocolly/colly/v2"
	datautils "github.com/soumitsalman/data-utils"
)

const (
	USER_AGENT = "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.10136"
)

type WebArticle struct {
	URL         string `json:"url,omitempty"`
	Source      string `json:"source,omitempty"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	Author      string `json:"author,omitempty"`
	PublishDate int64  `json:"created,omitempty"`
	Category    string `json:"category,omitempty"`
	Comments    int    `json:"comments,omitempty"`
	Likes       int    `json:"likes,omitempty"`
}

func (c *WebArticle) ToString() string {
	// TODO: remove. temp for debugging
	// c.Body = datautils.TruncateTextWithEllipsis(c.Body, 150)
	json_data, _ := json.MarshalIndent(c, "", "\t")
	return fmt.Sprint(string(json_data))
}

type WebArticleCollector struct {
	articles map[string]*WebArticle
	// supportedUrls []string
	collector   *colly.Collector
	sitemap_url string
}

// func (c *WebArticleCollector) getOrCreateOne(url string) *WebArticle {
// 	article, ok := c.articles[url]
// 	// create one if it doesnt exist
// 	if !ok {
// 		article = &WebArticle{URL: url}
// 		c.articles[url] = article
// 	}
// 	return article
// }

func (c *WebArticleCollector) Exists(url string) bool {
	return c.articles[url] != nil
}

func (c *WebArticleCollector) Get(url string) *WebArticle {
	article, ok := c.articles[url]
	// create one if it doesnt exist
	if !ok {
		return nil
	}
	return article
}

func (c *WebArticleCollector) ListAll() []*WebArticle {
	_, articles := datautils.MapToArray[string, *WebArticle](c.articles)
	return articles
}

// this function will return an instance of an extracted WebArticle if the url contains an HTML body
// for RSS feeds and Sitemaps it may return nil depending on the implementation since RSS feeds and sitemaps contain more than 1
func (c *WebArticleCollector) Collect(url string) *WebArticle {
	article, ok := c.articles[url]
	// check the cache
	if !ok {
		article = &WebArticle{URL: url}
		c.articles[url] = article
		c.collector.Visit(url)
	}
	return article
}

func (c *WebArticleCollector) CollectSite() []*WebArticle {
	c.collector.Visit(c.sitemap_url)
	return c.ListAll()
}
