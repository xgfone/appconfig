package store

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xgfone/go-tools/function"
	"github.com/xgfone/go-tools/types"
)

func init() {
	RegisterStore("memory", NewMemoryStore())
}

// memoryStore is the memory backend store, which is only used to test.
type memoryStore struct {
	sync.Mutex
	keys      map[string]map[int64]string
	callbacks map[string]map[string]string
	results   map[string]map[string][][3]string
}

// NewMemoryStore returns a new MemoryStore.
func NewMemoryStore() Store {
	m := &memoryStore{
		keys:      make(map[string]map[int64]string),
		callbacks: make(map[string]map[string]string),
		results:   make(map[string]map[string][][3]string),
	}

	return m
}

func (m *memoryStore) getLastestValue(ms map[int64]string) (string, error) {
	_len := len(ms)
	if _len == 0 {
		return "", ErrNotFound
	}
	vs := make([]int, 0, _len)
	for key := range ms {
		vs = append(vs, int(key))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(vs)))
	return ms[int64(vs[0])], nil
}

func (m *memoryStore) AppGetConfig(dc, env, app, key string, _time int64) (
	string, error) {
	m.Lock()
	defer m.Unlock()

	k := m.getKey(dc, env, app, key)
	vs := m.keys[k]
	if vs == nil {
		return "", ErrNotFound
	}
	if _time != 0 {
		if v, ok := vs[_time]; ok {
			return v, nil
		}
		return "", ErrNotFound
	}

	return m.getLastestValue(vs)
}

func (m *memoryStore) DeleteConfig(dc, env, app, key string, _time int64) error {
	if dc == "" {
		return ErrNotFound
	}

	m.Lock()
	defer m.Unlock()

	var prefix string
	if env == "" {
		prefix = m.getPrefix([]string{dc})
	} else if app == "" {
		prefix = m.getPrefix([]string{dc, env})
	} else if key == "" {
		prefix = m.getPrefix([]string{dc, env, app})
	} else if _time == 0 {
		prefix = m.getKey(dc, env, app, key)
		delete(m.keys, prefix)
		return nil
	} else {
		prefix = m.getKey(dc, env, app, key)
		if vs := m.keys[prefix]; vs != nil {
			delete(vs, _time)
		}
		return nil
	}

	keys := make([]string, 0, 8)
	for key := range m.keys {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	for _, key := range keys {
		delete(m.keys, key)
	}
	return nil
}

func (m *memoryStore) CreateDcAndEnv(dc, env string) error {
	m.Lock()
	defer m.Unlock()

	k := m.getKey(dc, env, "", "")
	m.keys[k] = nil
	return nil
}

func (m *memoryStore) GetAllDcAndEnvs() (map[string][]string, error) {
	m.Lock()
	defer m.Unlock()

	ms := make(map[string][]string, 2)
	for key := range m.keys {
		dc, env, _, _ := m.splitKey(key)
		if _, ok := ms[dc]; ok {
			if !function.InSlice(env, ms[dc]) {
				ms[dc] = append(ms[dc], env)
			}
		} else {
			ms[dc] = []string{env}
		}
	}
	return ms, nil
}

func (m *memoryStore) SetKeyValue(dc, env, app, key, value string) error {
	m.Lock()
	defer m.Unlock()

	now := time.Now().Unix()
	k := m.getKey(dc, env, app, key)
	if vs := m.keys[k]; vs != nil {
		vs[now] = value
	} else {
		m.keys[k] = map[int64]string{now: value}
	}
	return nil
}

func (m *memoryStore) GetAllApps(dc, env, search string, page, number int64) (
	int64, []string, error) {
	m.Lock()
	defer m.Unlock()

	isSearch := search != ""
	prefix := m.getPrefix([]string{dc, env})
	apps := make([]string, 0, 8)

	_keys, err := types.ToMapKeys(m.keys)
	if err != nil {
		return 0, nil, err
	}
	sort.Strings(_keys)

	for _, key := range _keys {
		if strings.HasPrefix(key, prefix) {
			_, _, app, _ := m.splitKey(key)
			if app == "" || function.InSlice(app, apps) {
				continue
			}

			if isSearch {
				if strings.Contains(app, search) {
					apps = append(apps, app)
				}
			} else {
				apps = append(apps, app)
			}
		}
	}

	total := int64(len(apps))
	start := (page - 1) * number
	end := start + number
	if start >= total {
		start = 0
		end = 0
	}
	if end >= total {
		end = total
	}
	return total, apps[start:end], nil
}

func (m *memoryStore) GetAllKeys(dc, env, app, search string, page,
	number int64) (int64, []string, error) {
	m.Lock()
	defer m.Unlock()

	isSearch := search != ""
	prefix := m.getPrefix([]string{dc, env, app})
	keys := make([]string, 0, 8)

	_keys, err := types.ToMapKeys(m.keys)
	if err != nil {
		return 0, nil, err
	}
	sort.Strings(_keys)

	for _, key := range _keys {
		if strings.HasPrefix(key, prefix) {
			_, _, _, _key := m.splitKey(key)
			if _key == "" || function.InSlice(_key, keys) {
				continue
			}

			if isSearch {
				if strings.Contains(_key, search) {
					keys = append(keys, _key)
				}
			} else {
				keys = append(keys, _key)
			}
		}
	}

	total := int64(len(keys))
	start := (page - 1) * number
	end := start + number
	if start >= total {
		start = 0
		end = 0
	}
	if end >= total {
		end = total
	}
	return total, keys[start:end], nil
}

func (m *memoryStore) GetAllValues(dc, env, app, key string, page, number, from,
	to int64) (int64, map[int64]string, error) {
	m.Lock()
	defer m.Unlock()

	key = m.getKey(dc, env, app, key)
	vs := m.keys[key]
	if vs == nil {
		return 0, nil, ErrNotFound
	}

	values := make(map[int64]string, 4)
	times := make([]int, 0, 4)
	for t, v := range vs {
		if (from == 0 && to == 0) || (from <= t && t <= to) {
			values[t] = v
			times = append(times, int(t))
		}
	}
	sort.Ints(times)

	total := int64(len(times))
	start := (page - 1) * number
	end := start + number
	if start >= total {
		start = 0
		end = 0
	}
	if end >= total {
		end = total
	}

	_values := make(map[int64]string)
	for _, t := range times[start:end] {
		_t := int64(t)
		_values[_t] = values[_t]
	}
	return total, _values, nil
}

func (m *memoryStore) AddCallback(dc, env, app, key, id, callback string) error {
	key = m.getKey(dc, env, app, key)
	m.Lock()
	if cs, ok := m.callbacks[key]; ok {
		cs[id] = callback
	} else {
		m.callbacks[key] = map[string]string{id: callback}
	}
	m.Unlock()
	return nil
}

func (m *memoryStore) GetCallback(dc, env, app, key string) (map[string]string, error) {
	key = m.getKey(dc, env, app, env)
	m.Lock()
	s, ok := m.callbacks[key]
	m.Unlock()
	if ok {
		return s, nil
	}
	return nil, ErrNotFound
}

func (m *memoryStore) DeleteCallback(dc, env, app, key, id string) error {
	key = m.getKey(dc, env, app, key)
	m.Lock()
	if cs, ok := m.callbacks[key]; ok {
		if id == "" {
			delete(cs, id)
			if len(cs) == 0 {
				delete(m.callbacks, key)
			}
		} else {
			delete(m.callbacks, key)
		}
	}
	m.Unlock()
	return nil
}

func (m *memoryStore) AddCallbackResult(dc, env, app, key, id, callback,
	result string) error {

	key = m.getKey(dc, env, app, key)
	value := [3]string{fmt.Sprintf("%d", time.Now().Unix()), callback, result}
	m.Lock()
	if cs, ok := m.results[key]; ok {
		if _, ok := cs[id]; ok {
			cs[id] = append(cs[id], value)
		} else {
			cs[id] = [][3]string{value}
		}
	} else {
		m.results[key] = map[string][][3]string{
			id: [][3]string{value},
		}
	}
	m.Unlock()
	return nil
}

func (m *memoryStore) GetCallbackResult(dc, env, app, key, id string) (
	[][3]string, error) {

	key = m.getKey(dc, env, app, key)
	m.Lock()
	if cs, ok := m.results[key]; ok {
		if v, ok := cs[id]; ok {
			end := len(v)
			start := end - 20
			if start < 0 {
				start = 0
			}
			return v[start:end], nil
		}
	}
	m.Unlock()
	return nil, ErrNotFound
}

func (m *memoryStore) Init(conf string) error {
	return nil
}

func (m *memoryStore) getPrefix(ss []string) string {
	ss = append(ss, "")
	return strings.Join(ss, "/")
}

func (m *memoryStore) getKey(dc, env, app, key string) string {
	return strings.Join([]string{dc, env, app, key}, "/")
}

func (m *memoryStore) splitKey(key string) (string, string, string, string) {
	vs := strings.Split(key, "/")
	return vs[0], vs[1], vs[2], vs[3]
}
