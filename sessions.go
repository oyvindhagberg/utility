package utility

import (
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"

	"github.com/icza/session"
)

// The HandlerFuncWithSession type has a similar signature to http.HandlerFunc,
// but adds a session parameter.
// Write your handler function with this signature to use sessions.
type HandlerFuncWithSession func(http.ResponseWriter, *http.Request, session.Session)

// WithSession extends the signature of http.HandlerFunc with a session parameter,
// and returns a regular http.HandlerFunc to be used with http.HandleFunc.
// When your function is called, it will always be passed a valid session object
// along with the Request and ResponseWriter.
func WithSession(h HandlerFuncWithSession) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := appengine.NewContext(r)
		if ptrNewStoreFunc == nil {
			log.Errorf(ctx, "ptrNewStoreFunc is unset; Can't create sessions.")
			http.Error(w, "500 Internal Serber Error", http.StatusInternalServerError)
			return
		}
		// Create a session manager
		options := session.CookieMngrOptions{}
		if appengine.IsDevAppServer() {
			// For testing purposes, we want cookies to be sent over HTTP too (not just HTTPS):
			options.AllowHTTP = true
		} else if r.TLS == nil {
			log.Errorf(ctx, "Can't use session cookies over http.")
			http.Error(w, "https is required.", http.StatusForbidden)
			return
		}
		sessmgr := session.NewCookieManagerOptions(ptrNewStoreFunc(ctx), &options)
		// deferring sessmgr.Close will ensure changes made to the session are
		// auto-saved in Memcache (and optionally in the Datastore):
		defer sessmgr.Close()
		// Get current session
		sess := sessmgr.Get(r)
		if sess == nil {
			// No session yet, let's create one and add it:
			sess = session.NewSessionOptions(
				&session.SessOptions{Timeout: sessionTimeout})
			sessmgr.Add(sess, w)
		}
		h(w, r, sess)
		// If it's time for garbage collection, run it as a task.
		// This is a way to avoid a cron job, which would require cron.yaml
		if time.Since(lastTimeOfGC) > timeBetweenGC {
			lastTimeOfGC = time.Now()
			task := taskqueue.NewPOSTTask(session_gc_path, url.Values{})
			taskqueue.Add(ctx, task, "") // add t to the default queue
		}
	})
}

func sessionGarbageCollectionWrapper(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Appengine-QueueName") != "" || appengine.IsDevAppServer() {
		ctx := appengine.NewContext(r)
		if ptrPurgeExpiredSessFromDSFunc != nil {
			log.Debugf(ctx, "Session garbage collection")
			ptrPurgeExpiredSessFromDSFunc(w, r)
		} else {
			log.Errorf(ctx, "Session purge func ptr is not set.")
		}
	} else {
		http.Error(w, "403 forbidden", http.StatusForbidden)
	}
}

func init() {
	lastTimeOfGC = time.Now()
	sessionTimeout = time.Duration(30) * time.Minute
	http.HandleFunc(session_gc_path, sessionGarbageCollectionWrapper)
}

var ptrPurgeExpiredSessFromDSFunc http.HandlerFunc
var ptrNewStoreFunc func(ctx context.Context) session.Store
var lastTimeOfGC time.Time
var sessionTimeout time.Duration

const session_gc_path = "/sessions/gc"
const timeBetweenGC time.Duration = time.Duration(30) * time.Minute
