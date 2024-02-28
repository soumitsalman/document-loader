#Web Collector
Simple utility for scraping blogs, news articles and sitemaps with more fidelity than some of the default libraries.
This is a wrapper on top existing libraries such as
- github.com/go-shiori/go-readability
- github.com/gocolly/colly/v2

## Usage

**Get Package:**
```
go get github.com/soumitsalman/web-collector
```

**Import:**
```
import (
	"github.com/soumitsalman/web-collector/collectors"
)
```

**Collecting One-off URLs:**
```
func main() {
	urls := []string{
		"https://kennybrast.medium.com/planning-a-successful-devops-strategy-for-a-fortune-200-enterprise-56304f1e28a8",
		"https://medium.com/@bohane.michael/navigating-risk-in-investment-fbbec34acd5f",
		"https://mymoneychronicles.medium.com/5-underrated-michael-jackson-songs-dfb6f8b08bb9",
		"https://thehackernews.com/2024/02/new-idat-loader-attacks-using.html",
		"https://thehackernews.com/2024/02/microsoft-releases-pyrit-red-teaming.html",
		"https://blogs.scientificamerican.com/at-scientific-american/systems-analysis-look-back-1966-scientific-american-article/",
		"https://www.scientificamerican.com/article/even-twilight-zone-coral-reefs-arent-safe-from-bleaching/",
		"https://www.scientificamerican.com/blog/at-scientific-american/reception-on-capitol-hill-will-celebrate-scientific-americans-cities-issue/",
		"https://blogs.scientificamerican.com/at-scientific-american/reception-on-capitol-hill-will-celebrate-scientific-americans-cities-issue/",
	}

	collector := collectors.NewDefaultCollector()
	for _, url := range urls {
		fmt.Println(collector.Collect(url).ToString())
	}
}
```
**Scraping From Sitemaps:**
```
func main() {

    // built-in scrapper for YC's hackernews.com topstories.json
    site_collector := collectors.NewYCHackerNewsSiteCollector(2)    
    // built-in sitemap scrapper for thehackersnews.com
	// site_collector := collectors.NewTheHackersNewsSiteCollector(7)
    // built-in scrapper for Medium's sitemap
	// site_collector := collectors.NewMediumSiteCollector(2)
    
    // the integer value refers to indicating that the collector will collect posts from the last N number days 

	for _, article := range site_collector.CollectSite() {
		fmt.Println(article.ToString())
	}
}

```


