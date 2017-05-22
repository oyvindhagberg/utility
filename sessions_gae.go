// +build appengine

package utility

import "github.com/icza/session"

func init() {
	ptrNewStoreFunc = session.NewMemcacheStore
	ptrPurgeExpiredSessFromDSFunc = session.PurgeExpiredSessFromDSFunc("")
}
