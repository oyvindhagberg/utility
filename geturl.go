package utility

import (
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type webCache struct {
	URL     string
	Content []byte    `datastore:",noindex"`
	Time    time.Time `datastore:",noindex"`
}

const webCacheEntityName string = "WebCache"

// Geturl performs an HTTP GET request and caches the result to the datastore
// so you can safely retrieve the same url many times over without
// hammering a server or depleting a quota.
func Geturl(ctx context.Context, url string, ttl time.Duration) ([]byte, error) {
	// Try to read from cache
	query := datastore.NewQuery(webCacheEntityName).Filter("URL = ", url)
	t := query.Run(ctx)
	var cached webCache
	key, err := t.Next(&cached)
	if err == nil {
		age := time.Since(cached.Time)
		if age <= ttl {
			log.Debugf(ctx, "Cache hit: %s", url)
			return cached.Content, nil
		}
		log.Debugf(ctx, "Cache hit: %s, expired. Age: %s", url, age.String())
	}
	// No cached version, perform an http request
	log.Debugf(ctx, "Cache miss: %s", url)
	//client := urlfetch.Client(ctx)
	client := &http.Client{
		Transport: &urlfetch.Transport{
			Context: ctx,
			// https://issuetracker.google.com/issues/35900087
			AllowInvalidServerCertificate: appengine.IsDevAppServer(),
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return []byte{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}, err
	}
	// Write the result to the cache
	if key == nil {
		key = datastore.NewIncompleteKey(ctx, webCacheEntityName, nil)
	}
	c := webCache{
		URL:     url,
		Content: body,
		Time:    time.Now(),
	}
	_, err = datastore.Put(ctx, key, &c)
	if err != nil {
		log.Errorf(ctx, err.Error())
		return []byte{}, err
	}
	// Return the contents
	return body, nil
}
