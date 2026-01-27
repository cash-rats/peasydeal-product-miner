package crawlworker

import "time"

type CrawlRequestedEventData struct {
	URL    string `json:"url"`
	OutDir string `json:"out_dir,omitempty"`
}

type CrawlRequestedEnvelope struct {
	EventName string                  `json:"event_name"`
	EventID   string                  `json:"event_id"`
	TS        time.Time               `json:"ts"`
	Data      CrawlRequestedEventData `json:"data"`
}
