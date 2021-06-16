package lib

import (
	"context"
	"log"
	"time"
)

// map[reqHost:username]lastRequestSuccessTime
var authorizedSource = make(map[string]time.Time, 1)

func LastRequestLogIndex(ctx context.Context) {
	updateInterval := 5 * time.Second
	ticker := time.NewTicker(updateInterval)
	for {
		ticker.Reset(updateInterval)
		select {
		case <-ctx.Done():
			log.Println("received a signal to cancel the service")
			return
		case <-ticker.C:
			for v, k := range authorizedSource {
				// no response for 2 minutes log again
				if k.Unix()+(60*2) < time.Now().Unix() {
					log.Printf("%s cache will be clean\n", v)
					delete(authorizedSource, v)
				}
			}
		}
	}
}
