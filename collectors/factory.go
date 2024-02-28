package collectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	"github.com/gocolly/colly/v2"
	datautils "github.com/soumitsalman/data-utils"
)

// var (
// 	TITLE_EXPR   = []string{".ArticleHeader__title", ".entry-title", "#entry-title", "#article-title", ".article-title", "[itemprop='headline']", "[data-testid=storyTitle]", "h1", "title"}
// 	BODY_EXPR    = []string{".article-content", "#article-content", ".article-container", "#article-container", "[itemprop=articleBody]", "#articlebody", ".c-entry-content", ".article-text", ".ArticleBody-articleBody", ".entry-content", "#entry-content", ".entry", "#entry", ".content", "#content", ".container", "article"}
// 	PUBDATE_EXPR = []string{".ArticleHeader__pub-date", "#ArticleHeader__pub-date", "[itemprop=datePublished]", "[data-testid=storyPublishDate]", "[data-testid='published-timestamp']"}
// 	AUTHOR_EXPR  = []string{".author", "[data-testid=authorName]", "[itemprop=author]", ".Author-authorName"}
// 	TAGS         = []string{".p-tags"}
// )

const (
	THE_HACKERSNEWS_SOURCE = "THE HACKERS NEWS"
	YC_HACKERNEWS_SOURCE   = "YC HACKER NEWS"
	MEDIUM_SOURCE          = "MEDIUM"
)

const (
	THE_HACKERSNEWS_SITE = "https://feeds.feedburner.com/TheHackersNews"
	YC_HACKERNEWS_SITE   = "https://hacker-news.firebaseio.com/v0/topstories.json"
	MEDIUM_SITE          = "https://medium.com/sitemap/sitemap.xml"
)

func ToPrettyJsonString(data any) string {
	val, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return ""
	}
	return string(val)
}

func NewCollector(sitemap_url string) *WebArticleCollector {
	return &WebArticleCollector{
		articles:    make(map[string]*WebArticle),
		collector:   colly.NewCollector(colly.UserAgent(USER_AGENT), colly.CacheDir("./.url-visit-cache")),
		sitemap_url: sitemap_url,
	}
}

// sitemap_url can be "" if the collector is not purposed for any specific sitemap scrapping
func NewDefaultCollector() *WebArticleCollector {
	web_collector := NewCollector("")
	web_collector.collector.OnResponse(func(r *colly.Response) {
		if article := readArticleFromResponse(r); article != nil {
			web_collector.articles[article.URL] = article
		}
	})
	return web_collector
}

// Collects from https://feeds.feedburner.com/TheHackersNews
func NewTheHackersNewsSiteCollector(days int) *WebArticleCollector {
	web_collector := NewCollector(THE_HACKERSNEWS_SITE) // collect all the links' body using another collector

	// matching entry items in the initial sitemap
	web_collector.collector.OnXML("//item", func(x *colly.XMLElement) {
		link := x.ChildText("/link")
		date, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", x.ChildText("/pubDate"))
		if err == nil && withinDateRange(date, days) && !web_collector.Exists(link) {
			web_collector.articles[link] = &WebArticle{
				URL:         link,
				Title:       x.ChildText("/title"),
				Author:      x.ChildText("/author"),
				PublishDate: date.Unix(),
				Source:      THE_HACKERSNEWS_SOURCE,
			}
			// now collect the body
			web_collector.collector.Visit(link)
		}
	})
	// just match the whole HTML for links that are being visited
	web_collector.collector.OnHTML("html", func(h *colly.HTMLElement) {
		if article := web_collector.Get(h.Request.URL.String()); article != nil {
			article.Body = readBodyFromResponse(h.Response)
		}
	})

	return web_collector
}

func NewMediumSiteCollector(days int) *WebArticleCollector {
	web_collector := NewCollector(MEDIUM_SITE)

	date_regex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	// this collects the overall site map of https://medium.com/sitemap/sitemap.xml
	web_collector.collector.OnXML("//sitemap/loc", func(x *colly.XMLElement) {
		link := x.Text
		date, err := time.Parse("2006-01-02", date_regex.FindString(link))
		// no interest in anything other than posts
		if err == nil && strings.Contains(link, "/posts/") && withinDateRange(date, days) {
			// this collects the sitemap for the posts
			web_collector.collector.Visit(link)
		}
	})

	// this is the sitemap for posts https://medium.com/sitemap/posts/2024/posts-2024-02-26.xml
	web_collector.collector.OnXML("//url", func(x *colly.XMLElement) {
		link := x.ChildText("/loc")
		date, err := time.Parse("2006-01-02", x.ChildText("/lastmod"))

		if err == nil && withinDateRange(date, days) && !web_collector.Exists(link) {
			web_collector.articles[link] = &WebArticle{
				URL:         link,
				PublishDate: date.Unix(),
				Source:      MEDIUM_SOURCE,
			}
			// now collect the body
			web_collector.collector.Visit(link)
		}
	})

	// this is the actual post. just match the whole HTML for links that are being visited
	web_collector.collector.OnHTML("html", func(h *colly.HTMLElement) {
		// get or create because sometime's the URLs change benignly
		if article := web_collector.Get(h.Request.URL.String()); article != nil {
			new_article := readArticleFromResponse(h.Response)
			article.Title = new_article.Title
			article.Body = new_article.Body
		}
	})

	return web_collector
}

func NewYCHackerNewsSiteCollector(days int) *WebArticleCollector {
	// https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty
	web_collector := NewCollector(YC_HACKERNEWS_SITE)

	web_collector.collector.OnResponse(func(r *colly.Response) {

		url := r.Request.URL.String()
		// visiting the topstories https://hacker-news.firebaseio.com/v0/topstories.json
		if url == web_collector.sitemap_url {
			// [ 9129911, 9129199, 9127761, 9128141, 9128264, 9127792, 9129248, 9127092, 9128367, ..., 9038733 ]
			var ids []int64
			if json.Unmarshal(r.Body, &ids) == nil {
				// decode successful, now visit these items
				// https://hacker-news.firebaseio.com/v0/item/8863.json
				datautils.ForEach(ids, func(item *int64) {
					web_collector.collector.Visit(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", *item))
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
				!web_collector.Exists(item_data.URL) { // item has NOT been explored already
				web_collector.articles[item_data.URL] = &WebArticle{
					URL:         item_data.URL,
					Title:       item_data.Title,
					Author:      item_data.Author,
					PublishDate: item_data.Time,
					Source:      YC_HACKERNEWS_SOURCE,
					Comments:    len(item_data.Kids),
					Likes:       item_data.Score,
				}
				// now collect the body
				web_collector.collector.Visit(item_data.URL)
			}
		} else {
			// it is a link to an actual story that is being visited
			// ideally all links that get to this should have an entry but links change benignly and this is done to avoid crash
			if article := web_collector.Get(r.Request.URL.String()); article != nil {
				article.Body = readBodyFromResponse(r)
			}
		}
	})
	return web_collector
}

func withinDateRange(date time.Time, range_days int) bool {
	// 1 is being added to get past some unknown bug
	return date.AddDate(0, 0, range_days+1).After(time.Now())
}

func readBodyFromResponse(resp *colly.Response) string {
	if raw_article, err := readability.FromReader(bytes.NewReader(resp.Body), resp.Request.URL); err == nil {
		return raw_article.TextContent
	}
	return ""
}

func readArticleFromResponse(resp *colly.Response) *WebArticle {
	if raw_article, err := readability.FromReader(bytes.NewReader(resp.Body), resp.Request.URL); err == nil {
		return &WebArticle{
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
		}
	}
	return nil
}

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
