package utility

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

func TestGeturl(t *testing.T) {
	if testing.Short() {
		return
	}

	// Preparation.
	// Need strong consistency since data will be read back right after being written.
	inst, err := aetest.NewInstance(&aetest.Options{StronglyConsistentDatastore: true})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	req, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := appengine.NewContext(req)
	const testURL string = "http://www.uio.no"
	const testContent string = "Universitetet i Oslo"

	// Perform a GET request, the content should be written to the datastore
	ttl := time.Duration(5) * time.Minute
	bytes1, err := Geturl(ctx, testURL, ttl)
	if err != nil {
		t.Fatalf("http request %s failed: %s", testURL, err.Error())
	}

	// Verify that we received the expected content
	str := string(bytes1)
	if !strings.Contains(str, testContent) {
		t.Fatalf("http request %s returned unexpected content\n", testURL)
	}

	// Verify that the content got written to the datastore
	query := datastore.NewQuery(webCacheEntityName).Filter("URL = ", testURL)
	tr := query.Run(ctx)
	var c webCache
	key, err := tr.Next(&c)
	if err != nil {
		t.Fatal("Cache write failed")
	}
	str = string(c.Content)
	if !strings.Contains(str, testContent) {
		t.Fatal("Cache write failed, incorrect content")
	}

	// Verify that the function will read content from the datastore
	// when the URL matches an object
	c.Content = []byte{1, 2, 3, 4}
	_, err = datastore.Put(ctx, key, &c)
	bytes2, err := Geturl(ctx, testURL, ttl)
	if !bytes.Equal(bytes2, c.Content) {
		t.Fatalf("Cache read returned the wrong content.\nWanted: %s\nGot: %s\n", c.Content, str)
	}

	// Verify that when TTL has expired, a new request is performed
	bytes3, err := Geturl(ctx, testURL, time.Duration(0))
	str = string(bytes3)
	if !strings.Contains(str, testContent) {
		t.Fatal("Read from cache even though TTL has expired.")
	}

	// Verify that the updated content overwrote the previous
	query = datastore.NewQuery(webCacheEntityName).Filter("URL = ", testURL)
	tr = query.Run(ctx)
	_, err = tr.Next(&c)
	if err != nil {
		t.Fatal("The cached object mysteriously disappeared")
	}
	str = string(c.Content)
	if !strings.Contains(str, testContent) {
		t.Fatal("Cache write failed when TTL was expired, incorrect content")
	}
}
