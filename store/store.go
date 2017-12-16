package store

import (
	"fmt"
)

var (
	backends = make(map[string]Store, 2)
)

var (
	// ErrNotFound is returned when the record does not exist.
	ErrNotFound = fmt.Errorf("not found")

	// ErrExist is returned when the record has existed.
	ErrExist = fmt.Errorf("has existed")

	// ErrNoDcAndEnv is returned when there is no dc and evn.
	ErrNoDcAndEnv = fmt.Errorf("no dc and env")
)

// RegisterStore registers a backend store.
func RegisterStore(name string, store Store) {
	if _, ok := backends[name]; ok {
		panic(fmt.Errorf("The backend store '%s' has been registered", name))
	}
	backends[name] = store
}

// GetStore returns a backend store named name.
//
// Return nil if the backend store named name does not exist.
func GetStore(name string) Store {
	return backends[name]
}

// GetStringPage returns the content in the page-th page from result.
//
// result is the whole result. page is the ith page, which begins at 1.
// And number is the number of each page. For example,
//
//     GetPage([]string{"a", "b", "c", "d", "e", "f", "g"}, 2, 3)
//     // ["d", "e", "f"]
func GetStringPage(result []string, page, number int64) []string {
	total := int64(len(result))
	start := (page - 1) * number
	end := start + number
	if start >= total {
		start = 0
		end = 0
	}
	if end >= total {
		end = total
	}
	return result[start:end]
}

// GetInt64Page is the same as GetStringPage, but for []int64
func GetInt64Page(result []int64, page, number int64) []int64 {
	total := int64(len(result))
	start := (page - 1) * number
	end := start + number
	if start >= total {
		start = 0
		end = 0
	}
	if end >= total {
		end = total
	}
	return result[start:end]
}

// Store is the interface of the backend store.
type Store interface {
	Init(conf string) error

	// AppGetConfig is used by the app to get the value of the key in APP.
	//
	// If the time is 0 or negative, it should return the latest value.
	// Or it should return the value at the provided time.
	AppGetConfig(dc, env, app, key string, _time int64) (v string, err error)

	// CreateDcAndEnv creates the new dc and env.
	//
	// If the dc and evn has existed, it either returns ErrExist or do nothing,
	// Which is determined by the implementation.
	CreateDcAndEnv(dc, env string) error

	// DeleteConfig deletes the config by the provided information.
	//
	//   1. dc must not be empty.
	//   2. If env is "", it should delete the whole dc.
	//   3. If app is "", it should delete the whole env.
	//   4. If key is "", it should delete the whole app.
	//   5. If _time is 0 or negative, it should delete the whole key.
	//
	// Notice: you can consider them as "/dc/env/app/key/_time".
	DeleteConfig(dc, env, app, key string, _time int64) error

	// GetAllDcAndEnvs returns all dc and env. The key is dc, and the value is
	// the all envs in the dc.
	GetAllDcAndEnvs() (map[string][]string, error)

	// SetKeyValue sets the key-value in dc, evn and app.
	//
	// If the key has not existed, it will create it; Or append it with a new
	// timestamp.
	SetKeyValue(dc, env, app, key, value string) error

	// GetAllApps returns the names of all apps in dc and env.
	//
	// If search is not "", it will return those apps the name of which contains
	// search.
	//
	// page is the ith page, and number the number of the apps in one page.
	GetAllApps(dc, env, search string, page, number int64) (int64, []string, error)

	// GetAllKeys returns the names of all keys in dc, env and app.
	//
	// If search is not "", it will return those keys the name of which contains
	// search.
	//
	// page is the ith page, and number the number of the apps in one page.
	GetAllKeys(dc, env, app, search string, page, number int64) (int64, []string, error)

	// GetAllValues returns the values of all keys in dc, env and app.
	//
	// page is the ith page, and number the number of the apps in one page.
	//
	// from and to is the start and end time to filte the values.
	GetAllValues(dc, env, app, key string, page, number, from, to int64) (int64, map[int64]string, error)

	///////////////////////////////////////////////////////////////////////////
	// Callback Notification

	// AddCallback adds a callback notification for a certain key of app
	// in dc and env.
	AddCallback(dc, env, app, key, id, callback string) error

	// GetCallback returns all the callback notifications of a key of app
	// in dc and env.
	//
	// The result is a map, the key of which is the registered id,
	// and the value of that is the callback value.
	GetCallback(dc, env, app, key string) (map[string]string, error)

	// DeleteCallback deletes all the callback notifications of the key of app
	// in dc and env.
	//
	// If id is not "", only delete the callback notification identified by id.
	DeleteCallback(dc, env, app, key, id string) error
}
