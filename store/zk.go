package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/xgfone/go-tools/lifecycle"
	"github.com/xgfone/go-tools/types"
	"github.com/xgfone/log"
)

func init() {
	RegisterStore("zk", NewZkStore())
}

// ZkLoggerFunc is a function wrapper of Zk Logger, which converts a function
// to the type of zk.Logger.
type ZkLoggerFunc func(string, ...interface{})

// Printf implements the interface of zk.Logger.
func (l ZkLoggerFunc) Printf(format string, args ...interface{}) {
	l(format, args...)
}

// NewZkConn returns a new zk.Conn.
func NewZkConn(addrs []string, timeout int, logger ...zk.Logger) (c *zk.Conn,
	err error) {

	c, ev, err := zk.Connect(addrs, time.Duration(timeout)*time.Second)
	if err != nil {
		return
	}

	if len(logger) > 0 && logger[0] != nil {
		c.SetLogger(logger[0])
	}

	lifecycle.Register(func() { c.Close() })
	go func() {
		for {
			if _, ok := <-ev; !ok {
				lifecycle.Stop()
				return
			}
		}
	}()

	return
}

// zkStore is the ZooKeeper store backend.
type zkStore struct {
	root  string
	acl   []zk.ACL
	flags int32
	zk    *zk.Conn
}

// NewZkStore returns a new ZooKeeper store backend.
func NewZkStore() Store {
	return &zkStore{
		acl: []zk.ACL{zk.ACL{Perms: 0x1f, Scheme: "world", ID: "anyone"}},
	}
}

func (z *zkStore) path(f string, args ...interface{}) string {
	path := fmt.Sprintf(f, args...)
	if z.root != "/" {
		return fmt.Sprintf("%s/config%s", z.root, path)
	}
	return "/config" + path
}

func (z *zkStore) cbPath(f string, args ...interface{}) string {
	path := fmt.Sprintf(f, args...)
	if z.root != "/" {
		return fmt.Sprintf("%s/callback%s", z.root, path)
	}
	return "/callback" + path
}

func (z *zkStore) cbResultPath(f string, args ...interface{}) string {
	path := fmt.Sprintf(f, args...)
	if z.root != "/" {
		return fmt.Sprintf("%s/cbresult%s", z.root, path)
	}
	return "/cbresult" + path
}

func (z *zkStore) Init(conf string) (err error) {
	var adds []string
	var timeout = 3
	var root = "/"

	if conf == "" {
		return fmt.Errorf("no zk config")
	}

	ss := strings.Split(conf, "&")
	if len(ss) == 1 {
		adds = strings.Split(conf, ",")
	} else {
		for _, s := range ss {
			vs := strings.SplitN(s, "=", 2)
			if len(vs) != 2 {
				return fmt.Errorf("the format of zk config is wrong: %s", s)
			}

			switch vs[0] {
			case "addr":
				adds = strings.Split(vs[1], ",")
			case "root":
				root = strings.TrimRight(vs[1], "/")
				if root == "" {
					root = "/"
				}
				if root[0] != '/' {
					return fmt.Errorf("the root path does not start with /")
				}
			case "timeout":
				if timeout, err = types.ToInt(vs[1]); err != nil {
					return
				}
			}
		}
	}

	if len(adds) == 0 {
		return fmt.Errorf("no zk addr")
	}

	z.zk, err = NewZkConn(adds, timeout, ZkLoggerFunc(log.Infof))
	z.root = root
	return
}

// AppGetConfig is used by the app to get the value of the key in APP.
//
// If the time is 0 or negative, it should return the latest value.
// Or it should return the value at the provided time.
func (z *zkStore) AppGetConfig(dc, env, app, key string, _time int64) (v string,
	err error) {

	if _time > 0 {
		path := z.path("/%s/%s/%s/%s/%d", dc, env, app, key, _time)
		data, _, err := z.zk.Get(path)
		switch err {
		case nil:
			return string(data), nil
		case zk.ErrNoNode:
			return "", ErrNotFound
		default:
			return "", err
		}
	}

	// Get all the childrens, that's the timestamps.
	path := z.path("/%s/%s/%s/%s", dc, env, app, key)
	cs, _, err := z.zk.Children(path)
	switch err {
	case nil:
	case zk.ErrNoNode:
		return "", ErrNotFound
	default:
		return
	}

	if len(cs) == 0 {
		return "", ErrNotFound
	}

	// Get the lastest timestamp
	sort.Sort(sort.Reverse(sort.StringSlice(cs)))
	path = fmt.Sprintf("%s/%s", path, cs[0])

	// Get the data
	data, _, err := z.zk.Get(path)
	switch err {
	case nil:
		return string(data), nil
	case zk.ErrNoNode:
		return "", ErrNotFound
	default:
		return
	}
}

// CreateDcAndEnv creates the new dc and env.
func (z *zkStore) CreateDcAndEnv(dc, env string) error {
	path := z.path("/%s", dc)
	if err := z.ensurePath(path); err != nil {
		return err
	}

	path = fmt.Sprintf("%s/%s", path, env)
	return z.ensurePath(path)
}

// DeleteConfig deletes the config by the provided information.
//
//   1. dc must not be empty.
//   2. If env is "", it should delete the whole dc.
//   3. If app is "", it should delete the whole env.
//   4. If key is "", it should delete the whole app.
//   5. If _time is 0 or negative, it should delete the whole key.
//
// Notice: you can consider them as "/dc/env/app/key/_time".
func (z *zkStore) DeleteConfig(dc, env, app, key string, _time int64) error {
	if dc == "" {
		return fmt.Errorf("dc is empty")
	}
	path := z.path("/%s", dc)

	if env != "" {
		path = fmt.Sprintf("%s/%s", path, env)
		if app != "" {
			path = fmt.Sprintf("%s/%s", path, app)
			if key != "" {
				path = fmt.Sprintf("%s/%s", path, key)
				if _time > 0 {
					path = fmt.Sprintf("%s/%d", path, _time)
				}
			}
		}
	}

	return z.deletePathRecursion(path)
}

func (z *zkStore) deletePathRecursion(path string) error {
	// Get all the children of the current path.
	cs, _, err := z.zk.Children(path)
	if err != nil {
		return err
	}

	// Delete the children recursively.
	for _, child := range cs {
		_path := fmt.Sprintf("%s/%s", path, child)
		if err := z.deletePathRecursion(_path); err != nil {
			return err
		}
	}

	// Delete the current path.
	if err := z.zk.Delete(path, -1); err != zk.ErrNoNode {
		return err
	}
	return nil
}

// GetAllDcAndEnvs returns all dc and env. The key is dc, and the value is
// the all envs in the dc.
func (z *zkStore) GetAllDcAndEnvs() (map[string][]string, error) {
	dcs, _, err := z.zk.Children(z.root)
	if err != nil {
		return nil, err
	}

	if len(dcs) == 0 {
		return map[string][]string{}, nil
	}
	m := make(map[string][]string, len(dcs))
	for _, dc := range dcs {
		path := z.path("/%s", dc)
		envs, _, err := z.zk.Children(path)
		if err != nil {
			return nil, err
		}
		m[dc] = envs
	}

	return m, nil
}

// SetKeyValue sets the key-value in dc, evn and app.
//
// If the key has not existed, it will create it; Or append it with a new
// timestamp.
func (z *zkStore) SetKeyValue(dc, env, app, key, value string) error {
	data := []byte(value)

	// First retry to set the value.
	// If there is not the parent node, create it, then retry to set the value.
	path := z.path("/%s/%s/%s/%s/%d", dc, env, app, key, time.Now().Unix())
	if _, err := z.zk.Create(path, data, z.flags, z.acl); err == nil {
		return nil
	} else if err != zk.ErrNoNode {
		return err
	}

	// Ensure the path /dc/env.
	p := z.path("/%s/%s", dc, env)
	if ok, _, err := z.zk.Exists(p); err != nil {
		return err
	} else if !ok {
		return ErrNoDcAndEnv
	}

	// Ensure the path /dc/env/app
	p = fmt.Sprintf("%s/%s", p, app)
	if err := z.ensurePath(p); err != nil {
		return err
	}

	// Ensure the path /dc/env/app/key
	p = fmt.Sprintf("%s/%s", p, key)
	if err := z.ensurePath(p); err != nil {
		return err
	}

	// Set the value of the path repeatedly.
	_, err := z.zk.Create(path, data, z.flags, z.acl)
	return err
}

func (z *zkStore) ensurePath(path string) (err error) {
	ok, _, err := z.zk.Exists(path)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	_, err = z.zk.Create(path, nil, z.flags, z.acl)
	return err
}

// GetAllApps returns the names of all apps in dc and env.
//
// If search is not "", it will return those apps the name of which contains
// search.
//
// page is the ith page, and number the number of the apps in one page.
func (z *zkStore) GetAllApps(dc, env, search string, page, number int64) (int64,
	[]string, error) {

	path := z.path("/%s/%s", dc, env)
	cs, _, err := z.zk.Children(path)
	if err == zk.ErrNoNode {
		return 0, nil, ErrNotFound
	} else if err != nil {
		return 0, nil, err
	}

	isSearch := search != ""
	apps := make([]string, 0, len(cs))
	for _, c := range cs {
		if isSearch {
			if strings.Contains(c, search) {
				apps = append(apps, c)
			}
		} else {
			apps = append(apps, c)
		}
	}

	return int64(len(apps)), GetStringPage(apps, page, number), nil
}

// GetAllKeys returns the names of all keys in dc, env and app.
//
// If search is not "", it will return those keys the name of which contains
// search.
//
// page is the ith page, and number the number of the apps in one page.
func (z *zkStore) GetAllKeys(dc, env, app, search string, page, number int64) (
	int64, []string, error) {

	path := z.path("/%s/%s/%s", dc, env, app)
	cs, _, err := z.zk.Children(path)
	if err == zk.ErrNoNode {
		return 0, nil, ErrNotFound
	} else if err != nil {
		return 0, nil, err
	}

	isSearch := search != ""
	keys := make([]string, 0, len(cs))
	for _, c := range cs {
		if isSearch {
			if strings.Contains(c, search) {
				keys = append(keys, c)
			}
		} else {
			keys = append(keys, c)
		}
	}

	return int64(len(keys)), GetStringPage(keys, page, number), nil
}

// GetAllValues returns the values of all keys in dc, env and app.
//
// page is the ith page, and number the number of the apps in one page.
//
// from and to is the start and end time to filte the values.
func (z *zkStore) GetAllValues(dc, env, app, key string, page, number, from,
	to int64) (int64, map[int64]string, error) {

	path := z.path("/%s/%s/%s/%s", dc, env, app, key)
	cs, _, err := z.zk.Children(path)
	if err == zk.ErrNoNode {
		return 0, nil, ErrNotFound
	} else if err != nil {
		return 0, nil, err
	}

	sort.Strings(cs)
	times := make([]int64, 0, number)
	for _, c := range cs {
		v, _ := types.ToInt64(c)
		if v == 0 {
			continue
		}

		if from == 0 && to == 0 {
			times = append(times, v)
			continue
		}

		if from <= v && v <= to {
			times = append(times, v)
		}
	}

	total := int64(len(times))
	times = GetInt64Page(times, page, number)
	values := make(map[int64]string, len(times))
	for _, t := range times {
		_path := fmt.Sprintf("%s/%d", path, t)
		data, _, err := z.zk.Get(_path)
		if err != nil {
			return 0, nil, err
		}
		values[t] = string(data)
	}

	return total, values, nil
}

func (z *zkStore) getCbPath(dc, env, app, key string) string {
	return z.cbPath("/%s|%s|%s|%s", dc, env, app, key)
}

func (z *zkStore) AddCallback(dc, env, app, key, id, callback string) error {
	path := z.getCbPath(dc, env, app, key)
	if err := z.ensurePath(path); err != nil {
		return err
	}

	path = fmt.Sprintf("%s/%s", path, id)
	_, err := z.zk.Create(path, []byte(callback), z.flags, z.acl)
	return err
}

func (z *zkStore) GetCallback(dc, env, app, key string) (map[string]string, error) {
	path := z.getCbPath(dc, env, app, key)
	cs, _, err := z.zk.Children(path)
	if err != zk.ErrNoNode {
		return nil, ErrNotFound
	}
	if len(cs) == 0 {
		return map[string]string{}, nil
	}

	result := make(map[string]string, len(cs))
	for _, c := range cs {
		data, _, err := z.zk.Get(fmt.Sprintf("%s/%s", path, c))
		if err != nil {
			return nil, err
		}
		result[c] = string(data)
	}

	return result, nil
}

func (z *zkStore) DeleteCallback(dc, env, app, key, id string) error {
	path := z.getCbPath(dc, env, app, key)
	if id != "" {
		path = fmt.Sprintf("%s/%s", path, id)
	}
	err := z.zk.Delete(path, -1)
	if err == zk.ErrNoNode {
		return nil
	}
	return err
}

func (z *zkStore) getCbResultPath(dc, env, app, key string) string {
	return z.cbResultPath("/%s|%s|%s|%s", dc, env, app, key)
}

func (z *zkStore) AddCallbackResult(dc, env, app, key, id, cb, r string) error {
	path := z.getCbResultPath(dc, env, app, key)
	if err := z.ensurePath(path); err != nil {
		return err
	}

	path = fmt.Sprintf("%s/%s", path, id)
	if err := z.ensurePath(path); err != nil {
		return err
	}

	path = fmt.Sprintf("%s/%d", path, time.Now().Unix())
	data, err := json.Marshal([]string{cb, r})
	if err != nil {
		return err
	}
	_, err = z.zk.Create(path, data, z.flags, z.acl)
	return err
}

func (z *zkStore) GetCallbackResult(dc, env, app, key, id string) (
	[][3]string, error) {

	path := z.getCbResultPath(dc, env, app, key)
	path = fmt.Sprintf("%s/%s", path, id)

	cs, _, err := z.zk.Children(path)
	if err != nil {
		return nil, err
	}
	if len(cs) > 20 {
		end := len(cs)
		start := end - 20
		if start < 0 {
			start = 0
		}
		cs = cs[start:end]
	}

	result := make([][3]string, len(cs))
	for i, c := range cs {
		data, _, err := z.zk.Get(fmt.Sprintf("%s/%s", path, c))
		if err != nil {
			return nil, err
		}
		var v []interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		result[i] = [3]string{c, v[0].(string), v[1].(string)}
	}

	return result, nil
}
