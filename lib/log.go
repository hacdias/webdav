package lib

import (
	"context"
	"log"
	"sync"
	"time"
)

// map[reqHost:username]lastRequestSuccessTime
var authorizedSource sync.Map

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
			authorizedSource.Range(func(k, v interface{}) bool {
				// no response for 2 minutes log again
				if v.(time.Time).Unix()+(60*2) < time.Now().Unix() {
					log.Printf("%s cache will be clean\n", k)
					authorizedSource.Delete(k)
				}
				return true
			})
		}
	}
}
