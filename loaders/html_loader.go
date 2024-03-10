package loaders

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	datautils "github.com/soumitsalman/data-utils"
	doc "github.com/soumitsalman/document-loader/document"
)

// var (
// 	TITLE_EXPR   = []string{".ArticleHeader__title", ".entry-title", "#entry-title", "#article-title", ".article-title", "[itemprop='headline']", "[data-testid=storyTitle]", "h1", "title"}
// 	BODY_EXPR    = []string{".article-content", "#article-content", ".article-container", "#article-container", "[itemprop=articleBody]", "#articlebody", ".c-entry-content", ".article-text", ".ArticleBody-articleBody", ".entry-content", "#entry-content", ".entry", "#entry", ".content", "#content", ".container", "article"}
// 	PUBDATE_EXPR = []string{".ArticleHeader__pub-date", "#ArticleHeader__pub-date", "[itemprop=datePublished]", "[data-testid=storyPublishDate]", "[data-testid='published-timestamp']"}
// 	AUTHOR_EXPR  = []string{".author", "[data-testid=authorName]", "[itemprop=author]", ".Author-authorName"}
// 	TAGS         = []string{".p-tags"}
// )

const (
	BODY_EXPR       = ".article-content, #article-content, .article-container, #article-container, [itemprop=articleBody], #articlebody, .article-text, .post, #post, .posts, #posts, .entry-content, #entry-content, .content, #content, article, body"
	BODY_EXPR_SHORT = ".post, .content, article, body"
)

const (
	ARTICLE = "article"
)

const (
	_CACHE_DIR    = "./.url-visit-cache"
	_MAX_TIME_OUT = 15 * time.Second
)

const (
	THE_HACKERSNEWS_SOURCE = "THE HACKERS NEWS"
	YC_HACKERNEWS_SOURCE   = "YC HACKER NEWS"
	MEDIUM_SOURCE          = "MEDIUM"
)

const (
	_THE_HACKERSNEWS_SITE = "https://thehackernews.com/news-sitemap.xml"
	_YC_HACKERNEWS_SITE   = "https://hacker-news.firebaseio.com/v0/topstories.json"
	_MEDIUM_SITE          = "https://medium.com/sitemap/sitemap.xml"
)

// //	GENERIC WEB SITE LOADER		////
// loader class for web links and sites
// the loaded content is cached
type WebLoader struct {
	articles  map[string]*doc.Document
	collector *colly.Collector
	config    *WebLoaderConfig
}

type WebLoaderConfig struct {
	Sitemap           string
	DisallowedFilters []string
	Timeout           time.Duration
}

func (c *WebLoader) inCache(url string) bool {
	return c.articles[url] != nil
}

func (c *WebLoader) Get(url string) *doc.Document {
	article, ok := c.articles[url]
	// create one if it doesnt exist
	if !ok {
		return nil
	}
	return article
}

func (c *WebLoader) ListAll() []*doc.Document {
	_, articles := datautils.MapToArray[string, *doc.Document](c.articles)
	return articles
}

// this function will return an instance of an extracted WebArticle if the url contains an HTML body
func (c *WebLoader) LoadDocument(url string) *doc.Document {
	article, ok := c.articles[url]
	// check the cache
	if !ok {
		article = &doc.Document{URL: url}
		c.articles[url] = article
		c.collector.Visit(url)
		c.collector.Wait()
	}
	return article
}

// this function will load all the documents from a sitemap or rss feed
func (c *WebLoader) LoadSite() []*doc.Document {
	c.collector.Visit(c.config.Sitemap)
	c.collector.Wait()
	return c.ListAll()
}

// // 	DIFFERENT LOADER FACTORIES		////
// internal blank instance
func internalNewLoader(config *WebLoaderConfig) *WebLoader {
	col := colly.NewCollector(
		colly.CacheDir(_CACHE_DIR),
		colly.DisallowedURLFilters(datautils.Transform(config.DisallowedFilters, func(rule *string) *regexp.Regexp { return regexp.MustCompile(*rule) })...),
	)
	col.SetRequestTimeout(_MAX_TIME_OUT)
	extensions.RandomUserAgent(col)

	return &WebLoader{
		articles:  make(map[string]*doc.Document),
		collector: col,
		config:    config,
	}
}

// sitemap_url can be "" if the collector is not purposed for any specific sitemap scrapping
func NewDefaultWebTextLoader(config *WebLoaderConfig) *WebLoader {
	web_collector := internalNewLoader(config)
	web_collector.collector.OnHTML("html", func(h *colly.HTMLElement) {
		if raw_article := readArticleFromResponse(h.Response); raw_article != nil {
			web_collector.articles[raw_article.URL] = raw_article
		}
	})
	return web_collector
}

func NewRedditLinkLoader() *WebLoader {
	web_collector := internalNewLoader(&WebLoaderConfig{
		DisallowedFilters: []string{
			`(?i)\.(png|jpeg|jpg|gif|webp|mp4|avi|mkv|mp3|wav|pdf)$`,
			`(\/\/v\.redd\.it)|(\/\/i\.redd\.it)|(\/\/www\.reddit\.com\/gallery)|(\/\/www\.youtube\.com)`,
		},
	})

	web_collector.collector.OnHTML("html", func(h *colly.HTMLElement) {
		if article := readArticleFromResponse(h.Response); article != nil {
			web_collector.articles[article.URL] = article
		}
	})
	return web_collector
}

// Loads articles from https://feeds.feedburner.com/TheHackersNews that have been posted in the last N days
func NewDefaultNewsSitemapLoader(days int, sitemap_url string) *WebLoader {
	web_collector := internalNewLoader(&WebLoaderConfig{
		Sitemap:           sitemap_url,
		DisallowedFilters: []string{`(?i)\.(png|jpeg|jpg|gif|webp|mp4|avi|mkv|mp3|wav|pdf)$`},
	})
	web_collector.collector.AllowURLRevisit = true

	// matching entry items in the initial sitemap
	web_collector.collector.OnXML("//url", func(x *colly.XMLElement) {
		link := x.ChildText("/loc")
		date := parseDate(x.ChildText("/news:news/news:publication_date"))
		if withinDateRange(date, days) && !web_collector.inCache(link) {
			web_collector.articles[link] = &doc.Document{
				URL:         link,
				PublishDate: date.Unix(),
				Title:       x.ChildText("/news:news/news:title"),
				Source:      x.ChildText("/news:news/news:publication/news:name"),
				Category:    x.ChildText("/news:news/news:keywords"),
				Kind:        ARTICLE,
			}
			// now collect the body
			x.Request.Visit(link)
		}

	})
	// just match the whole HTML for links that are being visited
	web_collector.collector.OnHTML(BODY_EXPR_SHORT, func(h *colly.HTMLElement) {
		if article := web_collector.Get(h.Request.URL.String()); article != nil {
			article.Body = readBodyFromResponse(h.Response)
		}
	})

	return web_collector
}

// loades medium posts from https://medium.com/sitemap/sitemap.xml that have been modified in the last N days
func NewMediumSiteLoader(days int) *WebLoader {
	web_collector := internalNewLoader(&WebLoaderConfig{
		Sitemap:           _MEDIUM_SITE,
		DisallowedFilters: []string{`(?i)\.(png|jpeg|jpg|gif|webp|mp4|avi|mkv|mp3|wav|pdf)$`},
	})
	web_collector.collector.AllowURLRevisit = true

	date_regex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	// this collects the overall site map of https://medium.com/sitemap/sitemap.xml
	web_collector.collector.OnXML("//sitemap/loc", func(x *colly.XMLElement) {
		link := x.Text
		date := parseDate(date_regex.FindString(link))
		// no interest in anything other than posts
		if strings.Contains(link, "/posts/") && withinDateRange(date, days) {
			// this collects the sitemap for the posts
			x.Request.Visit(link)
		}
	})

	// this is the sitemap for posts https://medium.com/sitemap/posts/2024/posts-2024-02-26.xml
	web_collector.collector.OnXML("//url", func(x *colly.XMLElement) {
		link := x.ChildText("/loc")
		date := parseDate(x.ChildText("/lastmod"))

		if withinDateRange(date, days) && !web_collector.inCache(link) {
			web_collector.articles[link] = &doc.Document{
				URL:         link,
				PublishDate: date.Unix(),
				Source:      MEDIUM_SOURCE,
				Kind:        ARTICLE,
			}
			// now collect the body
			x.Request.Visit(link)
		}
	})

	// this is the actual post. just match the whole stuff within article tag for links that are being visited
	web_collector.collector.OnHTML("html", func(h *colly.HTMLElement) {
		// get or create because sometime's the URLs change benignly
		if article := web_collector.Get(h.Request.URL.String()); article != nil {
			article.Body = readBodyFromResponse(h.Response)
		}
	})

	return web_collector
}

// loads story links from https://hacker-news.firebaseio.com/v0/topstories.json posted in the last N days
func NewYCHackerNewsSiteLoader() *WebLoader {
	// https://hacker-news.firebaseio.com/v0/topstories.json
	web_collector := internalNewLoader(&WebLoaderConfig{
		Sitemap:           _YC_HACKERNEWS_SITE,
		DisallowedFilters: []string{`(?i)\.(png|jpeg|jpg|gif|webp|mp4|avi|mkv|mp3|wav|pdf)$`},
	})
	web_collector.collector.AllowURLRevisit = true

	web_collector.collector.OnResponse(func(r *colly.Response) {
		url := r.Request.URL.String()
		// visiting the topstories https://hacker-news.firebaseio.com/v0/topstories.json
		if url == web_collector.config.Sitemap {
			// [ 9129911, 9129199, 9127761, 9128141, 9128264, 9127792, 9129248, 9127092, 9128367, ..., 9038733 ]
			var ids []int64
			if json.Unmarshal(r.Body, &ids) == nil {
				// decode successful, now visit these items
				// https://hacker-news.firebaseio.com/v0/item/8863.json
				datautils.ForEach(ids, func(item *int64) {
					r.Request.Visit(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", *item))
				})
			}
		} else if match, _ := regexp.MatchString(`https:\/\/hacker-news\.firebaseio\.com\/v0\/item\/\d+\.json`, url); match {
			// visiting the description/metadata of an item in the topstories
			var item_data struct {
				Author string  `json:"by"`
				Kids   []int64 `json:"kids"`
				Score  int     `json:"score"`
				Time   int64   `json:"time"`
				Title  string  `json:"title"`
				URL    string  `json:"url"`
				Type   string  `json:"type"`
			}
			if json.Unmarshal(r.Body, &item_data) == nil && // marshalling has to succeed
				item_data.Type == "story" && // type has to be story
				item_data.URL != "" && // it has to be legit URL and not a text
				!web_collector.inCache(item_data.URL) { // item has NOT been explored already
				web_collector.articles[item_data.URL] = &doc.Document{
					URL:         item_data.URL,
					Title:       item_data.Title,
					Author:      item_data.Author,
					PublishDate: item_data.Time,
					Source:      YC_HACKERNEWS_SOURCE,
					Comments:    len(item_data.Kids),
					Likes:       item_data.Score,
					Kind:        ARTICLE,
				}
				// now collect the body
				r.Request.Visit(item_data.URL)
			}
		}
	})

	web_collector.collector.OnHTML(BODY_EXPR, func(h *colly.HTMLElement) {
		if article := web_collector.Get(h.Request.URL.String()); article != nil {
			article.Body = readBodyFromResponse(h.Response)
		}
	})

	return web_collector
}

// //	INTERNAL UTILITY FUNCTIONS		////
func withinDateRange(date time.Time, range_days int) bool {
	// 1 is being added to get past some unknown bug
	return date.AddDate(0, 0, range_days+1).After(time.Now())
}

func parseDate(val string) time.Time {
	// Layouts for parsing the time strings
	layouts := []string{
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.Stamp,
		time.StampMilli,
		time.DateTime,
		time.DateOnly,
	}

	// Parse time strings with different layouts
	for _, layout := range layouts {
		if parsed_date, err := time.Parse(layout, val); err == nil {
			return parsed_date
		}
	}
	return time.Time{}
}

func readArticleFromResponse(resp *colly.Response) *doc.Document {
	if raw_article, err := readability.FromReader(bytes.NewReader(resp.Body), resp.Request.URL); err == nil {
		return &doc.Document{
			URL:   resp.Request.URL.String(),
			Title: raw_article.Title,
			Body:  raw_article.TextContent,
			PublishDate: func() int64 {
				if raw_article.PublishedTime != nil {
					return raw_article.PublishedTime.Unix()
				}
				return 0
			}(),
			Source: resp.Request.URL.Host,
			Kind:   ARTICLE,
		}
	}
	return nil
}

func readBodyFromResponse(resp *colly.Response) string {
	if raw_article, err := readability.FromReader(bytes.NewReader(resp.Body), resp.Request.URL); err == nil {
		return raw_article.TextContent
	}
	return ""
}

// func ToPrettyJsonString(data any) string {
// 	val, err := json.MarshalIndent(data, "", "\t")
// 	if err != nil {
// 		return ""
// 	}
// 	return string(val)
// }

// // adding a rule for each expr so that if the field is still empty it will assign a value
// // TITLE
// assignField(TITLE_EXPR, web_collector, func(article *WebArticle, value_str string) {
// 	if article.Title == "" {
// 		article.Title = value_str
// 	}
// })
// // BODY
// assignField(BODY_EXPR, web_collector, func(article *WebArticle, value_str string) {
// 	if article.Body == "" {
// 		article.Body = value_str
// 	}
// })
// // AUTHOR
// assignField(AUTHOR_EXPR, web_collector, func(article *WebArticle, value_str string) {
// 	if article.Author == "" {
// 		article.Author = value_str
// 	}
// })
// // PUBLISH DATE
// assignField(PUBDATE_EXPR, web_collector, func(article *WebArticle, value_str string) {
// 	if article.PublishDate == "" {
// 		article.PublishDate = value_str
// 	}
// })
// // TAGS
// assignField(TAGS, web_collector, func(article *WebArticle, value_str string) {
// 	if article.Category == "" {
// 		article.Category = value_str
// 	}
// })

// collects from https://blogs.scientificamerican.com/ URLs
// func NewScientificAmericanURLCollector() *WebArticleCollector {
// 	web_collector := newCollector("blogs.scientificamerican.com")

// 	// // URL
// 	// // var article WebArticle
// 	// web_collector.collector.OnRequest(func(r *colly.Request) {
// 	// 	web_collector.Articles[r.URL.String()] = &WebArticle{URL: r.URL.String()}
// 	// })
// 	// AUTHOR
// 	// <span itemprop="author" itemscope="" itemtype="http://schema.org/Person">
// 	web_collector.collector.OnHTML("[itemprop=author]", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Author = b.Text
// 	})
// 	// PUBLISHED DATE
// 	// <time itemprop="datePublished" content="2011-08-23">August 23, 2011</time>
// 	web_collector.collector.OnHTML("time[itemprop=datePublished]", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).PublishDate = b.Text
// 	})
// 	// TITLE
// 	// <h1 class="article-header__title t_article-title" itemprop="headline">Prescient but Not Perfect: A Look Back at a 1966 <em>Scientific American</em> Article on Systems Analysis</h1>
// 	web_collector.collector.OnHTML("h1", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Title = b.Text
// 	})
// 	// BODY
// 	// div[itemprop=articleBody]
// 	web_collector.collector.OnHTML("[itemprop=articleBody]", func(b *colly.HTMLElement) {
// 		// TODO: dont crop it
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Body = b.Text[:200]
// 	})
// 	// NUMBER OF COMMENTS
// 	// <a href="#comments">
// 	web_collector.collector.OnHTML("a[href=#comments]", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Comments = b.Text
// 	})
// 	return web_collector
// }

// Collects from thehackersnews.com URL
// func NewTheHackersNewsPostCollector() *WebArticleCollector {
// 	web_collector := newCollector("thehackersnews.com")

// 	// // URL
// 	// // var article WebArticle
// 	// web_collector.collector.OnRequest(func(r *colly.Request) {
// 	// 	web_collector.Articles[r.URL.String()] = &WebArticle{URL: r.URL.String()}
// 	// })
// 	// AUTHOR
// 	// <div class="post-body"><div itemprop="author"><meta content="The Hacker News" itemprop="name">
// 	web_collector.collector.OnHTML("div[itemprop=author] > meta[itemprop=name]", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Author = b.Attr("content")
// 	})
// 	// PUBLISH DATE
// 	// <div class="post-body"><meta content="2024-02-26T20:24:00+05:30" itemprop="datePublished">
// 	web_collector.collector.OnHTML("meta[itemprop='datePublished']", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).PublishDate = b.Attr("content")
// 	})
// 	// TITLE
// 	// <div class="post-body"><meta content="New IDAT Loader Attacks Using Steganography to Deploy Remcos RAT" itemprop="headline">
// 	web_collector.collector.OnHTML("meta[itemprop='headline']", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Title = b.Attr("content")
// 	})
// 	// BODY
// 	// <div class="post-body"><div id=articlebody>
// 	web_collector.collector.OnHTML("div#articlebody", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Body = b.Text[:200]
// 	})
// 	// TAGS
// 	// <div class="postmeta"><span class="p-tags">Steganography / Malware</span>
// 	web_collector.collector.OnHTML("span.p-tags", func(b *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(b.Request.URL.String()).Category = b.Text
// 	})
// 	return web_collector
// }

// func NewMediumPostCollector() *WebArticleCollector {
// 	web_collector := newCollector()

// 	// https://medium.com/towardsdev/reinventing-the-wheel-deploying-slack-ai-chat-bot-in-azure-part-1-589a9363ed5c
// 	// AUTHOR
// 	// <div class="post-body"><div itemprop="author"><meta content="The Hacker News" itemprop="name">
// 	web_collector.collector.OnHTML("[data-testid=authorName]", func(h *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(h.Request.URL.String()).Author = h.Text
// 	})
// 	// PUBLISH DATE
// 	// <div class="post-body"><meta content="2024-02-26T20:24:00+05:30" itemprop="datePublished">
// 	web_collector.collector.OnHTML("[data-testid=storyPublishDate]", func(h *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(h.Request.URL.String()).PublishDate = h.Text
// 	})
// 	// TITLE
// 	// <div class="post-body"><div id=articlebody>
// 	web_collector.collector.OnHTML("[data-testid=storyTitle]", func(h *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(h.Request.URL.String()).Title = h.Text
// 	})
// 	// BODY
// 	// <div class="post-body"><div id=articlebody>
// 	web_collector.collector.OnHTML("[class='mv mw fr be mx my mz na nb nc nd ne nf ng nh ni nj nk nl nm nn no np nq nr ns bj']", func(h *colly.HTMLElement) {
// 		article := web_collector.getOrCreateArticle(h.Request.URL.String())
// 		article.Body = fmt.Sprintf("%s\n%s", article.Body, h.Text)
// 	})
// 	web_collector.collector.OnHTML("p", func(h *colly.HTMLElement) {

// 		article := web_collector.getOrCreateArticle(h.Request.URL.String())
// 		article.Body = fmt.Sprintf("%s\n\n%s", article.Body, h.Text)
// 	})
// 	// LIKES
// 	// <div class="post-body"><meta content="2024-02-26T20:24:00+05:30" itemprop="datePublished">
// 	web_collector.collector.OnHTML("[class='pw-multi-vote-count l jw jx jy jz ka kb kc']", func(h *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(h.Request.URL.String()).Likes = h.Text
// 	})
// 	// COMMENTS
// 	// <div class="post-body"><meta content="New IDAT Loader Attacks Using Steganography to Deploy Remcos RAT" itemprop="headline">
// 	web_collector.collector.OnHTML("[class='pw-responses-count lf lg']", func(h *colly.HTMLElement) {
// 		web_collector.getOrCreateArticle(h.Request.URL.String()).Comments = h.Text
// 	})

// 	return web_collector
// }

// func assignField(expr_arr []string, web_collector *WebArticleCollector, assign_func func(article *WebArticle, value_str string)) {
// 	datautils.ForEach[string](expr_arr, func(expr *string) {
// 		web_collector.collector.OnHTML(*expr, func(b *colly.HTMLElement) {
// 			assign_func(web_collector.getOrCreateArticle(b.Request.URL.String()), b.Text)
// 		})
// 	})
// }
