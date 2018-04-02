package utility

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/icza/session"
	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest"
	"google.golang.org/appengine/datastore"
)

func TestSessions(t *testing.T) {
	if testing.Short() {
		return
	}

	// Preparation
	// Need strong consistency since data will be read back right after being written.
	inst, err := aetest.NewInstance(
		&aetest.Options{StronglyConsistentDatastore: true})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	sessionTimeout = time.Duration(1) * time.Second

	// Fake a request that will create a session
	// and store a value in it
	const key string = "someValue"
	const value int = 123
	req, err := inst.NewRequest("GET", "/whatever", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h := WithSession(
		func(w http.ResponseWriter, r *http.Request, s session.Session) {
			s.SetAttr(key, value)
		})
	h(w, req)
	if w.Code != 200 {
		t.Fatalf("http status %d:\n%v", w.Code, w.Body)
	}
	sessionCookie := w.HeaderMap["Set-Cookie"][0]

	// Fake a new request that attempts to read the value back
	req, err = inst.NewRequest("GET", "/whatever", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Cookie", sessionCookie)
	w = httptest.NewRecorder()
	h = WithSession(
		func(w http.ResponseWriter, r *http.Request, s session.Session) {
			v := s.Attr(key)
			i, ok := v.(int)
			if !ok || i != value {
				t.Fatal("Couldn't read back value from session.")
			}
		})
	h(w, req)
	if w.Code != 200 {
		t.Fatalf("http status %d:\n%v", w.Code, w.Body)
	}
	ctx := appengine.NewContext(req)

	// Verify that the datastore contains something
	{
		q := datastore.NewQuery("sess_").KeysOnly().Limit(100)
		var keys []*datastore.Key
		if keys, err = q.GetAll(ctx, nil); err != nil {
			// Datastore error.
			t.Fatalf("Failed to query datastore: %v", err)
		}
		if len(keys) == 0 {
			t.Fatal("The datastore is empty, should contain the session.")
		}
	}

	// Wait until the session must have timed out
	time.Sleep(sessionTimeout)

	// Verify that the session no longer exists
	// (This will also cause it to be deleted from the memcache and datastore)
	options := session.CookieMngrOptions{}
	sessmgr := session.NewCookieManagerOptions(
		ptrNewStoreFunc(ctx),
		&options)
	defer sessmgr.Close()
	sess := sessmgr.Get(req)
	if sess != nil {
		t.Fatal("The session wasn't invalidated after expiry")
	}

	// Create another session
	req, err = inst.NewRequest("GET", "/whatever", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h = WithSession(
		func(w http.ResponseWriter, r *http.Request, s session.Session) {
			s.SetAttr(key, value)
		})
	h(w, req)
	if w.Code != 200 {
		t.Fatalf("http status %d:\n%v", w.Code, w.Body)
	}

	// Wait until the session must have timed out
	time.Sleep(sessionTimeout)

	// Run garbage collection
	req, err = inst.NewRequest("GET", "/whatever", nil)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	sessionGarbageCollectionWrapper(w, req)

	// The datastore should be empty now
	var keys []*datastore.Key
	q := datastore.NewQuery("sess_").KeysOnly().Limit(100)
	if keys, err = q.GetAll(ctx, nil); err != nil {
		// Datastore error.
		t.Fatalf("Failed to query datastore: %v", err)
	}
	if len(keys) > 0 {
		t.Fatalf("The datastore should be empty after GC, but contains %d items",
			len(keys))
	}
}
