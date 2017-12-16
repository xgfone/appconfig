package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/xgfone/appconfig/store"
	"github.com/xgfone/go-tools/net2/http2"
)

var backend store.Store

// InitStore the backend store.
func InitStore(storeName, conf string) error {
	if backend = store.GetStore(storeName); backend == nil {
		return fmt.Errorf("no the backend store named %s", storeName)
	}
	return backend.Init(conf)
}

func init() {
	wrap := http2.ErrorHandler

	r := mux.NewRouter()
	v1 := r.PathPrefix("/v1").Subrouter().StrictSlash(true)

	// App Config
	v1.Handle("/app/{dc}/{env}/{app}/{key}", wrap(AppGetConfig)).Methods("GET")

	// Admin Config
	v1.Handle("/admin", wrap(CreateDcAndEnv)).Methods("POST")
	v1.Handle("/admin", wrap(GetAllDcAndEnvs)).Methods("GET")

	admin := v1.PathPrefix("/admin").Subrouter()
	admin.Handle("/{dc}/{env}/{app}/{key}", wrap(UploadConfig)).Methods("POST")

	admin.Handle("/{dc}/{env}", wrap(GetAllApps)).Methods("GET")
	admin.Handle("/{dc}/{env}/{app}", wrap(GetAllKeys)).Methods("GET")
	admin.Handle("/{dc}/{env}/{app}/{key}", wrap(GetAllValues)).Methods("GET")

	admin.Handle("/{dc}", wrap(DeleteDc)).Methods("DELETE")
	admin.Handle("/{dc}/{env}", wrap(DeleteEnv)).Methods("DELETE")
	admin.Handle("/{dc}/{env}/{app}", wrap(DeleteApp)).Methods("DELETE")
	admin.Handle("/{dc}/{env}/{app}/{key}", wrap(DeleteKey)).Methods("DELETE")

	handler = r
}

func renderError(w http.ResponseWriter, err error) error {
	switch err {
	case nil:
	case store.ErrExist:
		w.WriteHeader(http.StatusNotAcceptable)
	case store.ErrNotFound:
		w.WriteHeader(http.StatusNotFound)
	case store.ErrNoDcAndEnv:
		return http2.String(w, http.StatusBadRequest, "no dc and env")
	default:
		logger.Errorf("Get an error: %s", err)
		if e, ok := err.(http2.HTTPError); ok {
			return http2.Error(w, e, e.Code)
		}
		return http2.Error(w, err)
	}
	return nil
}

// AppGetConfig returns the app config information.
//
// This interface is only accessed by the app.
func AppGetConfig(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	t, err := http2.GetQueryInt64(query, "time")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}

	vs := mux.Vars(r)

	v, err := backend.AppGetConfig(vs["dc"], vs["env"], vs["app"], vs["key"], t)
	if err == nil {
		return http2.String(w, http.StatusOK, "%s", v)
	}
	return renderError(w, err)
}

// CreateDcAndEnv create the new dc and env.
func CreateDcAndEnv(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()
	dc := http2.GetQuery(query, "dc")
	if dc == "" {
		return http2.String(w, http.StatusBadRequest, "missing dc")
	}

	env := http2.GetQuery(query, "env")
	if env == "" {
		return http2.String(w, http.StatusBadRequest, "missing env")
	}

	err := backend.CreateDcAndEnv(dc, env)
	printLog(err, "create dc=%s, env=%s", dc, env)
	return renderError(w, err)
}

// GetAllDcAndEnvs returns all dcs and envs.
func GetAllDcAndEnvs(w http.ResponseWriter, r *http.Request) error {
	v, err := backend.GetAllDcAndEnvs()
	if err != nil {
		return renderError(w, err)
	}
	return http2.JSON(w, http.StatusOK, v)
}

// UploadConfig uploads the app config information.
func UploadConfig(w http.ResponseWriter, r *http.Request) error {
	v, err := http2.GetBody(r)
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}

	vs := mux.Vars(r)
	err = backend.SetKeyValue(vs["dc"], vs["env"], vs["app"], vs["key"], string(v))
	printLog(err, "Upload the app config, dc=%s, env=%s, app=%s, key=%s",
		vs["dc"], vs["env"], vs["app"], vs["key"])
	return renderError(w, err)
}

// GetAllApps returns all apps in dc and env.
func GetAllApps(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()

	page, err := http2.GetQueryInt64(query, "page")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}
	if page < 1 {
		page = 1
	}

	size, err := http2.GetQueryInt64(query, "size")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}
	if size < 1 {
		size = 20
	}

	search := http2.GetQuery(query, "search")
	vs := mux.Vars(r)
	total, v, err := backend.GetAllApps(vs["dc"], vs["env"], search, page, size)
	if err != nil {
		return renderError(w, err)
	}
	return http2.JSON(w, http.StatusOK, map[string]interface{}{"total": total, "apps": v})
}

// GetAllKeys returns all keys in dc, env and app.
func GetAllKeys(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()

	page, err := http2.GetQueryInt64(query, "page")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}
	if page < 1 {
		page = 1
	}

	size, err := http2.GetQueryInt64(query, "size")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}
	if size < 1 {
		size = 20
	}

	search := http2.GetQuery(query, "search")
	vs := mux.Vars(r)
	total, v, err := backend.GetAllKeys(vs["dc"], vs["env"], vs["app"], search,
		page, size)
	if err != nil {
		return renderError(w, err)
	}
	return http2.JSON(w, http.StatusOK, map[string]interface{}{"total": total, "keys": v})
}

// GetAllValues returns all values of the key in dc, env and app.
func GetAllValues(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query()

	page, err := http2.GetQueryInt64(query, "page")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}
	if page < 1 {
		page = 1
	}

	size, err := http2.GetQueryInt64(query, "size")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}
	if size < 1 {
		size = 20
	}

	from, err := http2.GetQueryInt64(query, "from")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}

	to, err := http2.GetQueryInt64(query, "to")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}

	vs := mux.Vars(r)
	total, v, err := backend.GetAllValues(vs["dc"], vs["env"], vs["app"],
		vs["key"], page, size, from, to)
	if err != nil {
		return renderError(w, err)
	}
	return http2.JSON(w, http.StatusOK, map[string]interface{}{"total": total, "values": v})
}

// DeleteDc deletes the whole dc.
func DeleteDc(w http.ResponseWriter, r *http.Request) (err error) {
	vs := mux.Vars(r)
	err = backend.DeleteConfig(vs["dc"], "", "", "", 0)
	printLog(err, "Delete dc=%s", vs["dc"])
	return renderError(w, err)
}

// DeleteEnv deletes the whole env.
func DeleteEnv(w http.ResponseWriter, r *http.Request) (err error) {
	vs := mux.Vars(r)
	err = backend.DeleteConfig(vs["dc"], vs["env"], "", "", 0)
	printLog(err, "Delete dc=%s, env=%s", vs["dc"], vs["env"])
	return renderError(w, err)
}

// DeleteApp deletes the whole app.
func DeleteApp(w http.ResponseWriter, r *http.Request) (err error) {
	vs := mux.Vars(r)
	err = backend.DeleteConfig(vs["dc"], vs["env"], vs["app"], "", 0)
	printLog(err, "Delete dc=%s, env=%s, app=%s", vs["dc"], vs["env"], vs["app"])
	return renderError(w, err)
}

// DeleteKey deletes the whole key.
func DeleteKey(w http.ResponseWriter, r *http.Request) (err error) {
	query := r.URL.Query()
	t, err := http2.GetQueryInt64(query, "time")
	if err != nil {
		return http2.Error(w, err, http.StatusBadRequest)
	}

	vs := mux.Vars(r)
	err = backend.DeleteConfig(vs["dc"], vs["env"], vs["app"], vs["key"], t)
	printLog(err, "Delete dc=%s, env=%s, app=%s, key=%s, time=%d", vs["dc"],
		vs["env"], vs["app"], vs["key"], t)
	return renderError(w, err)
}
