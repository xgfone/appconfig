package store

import (
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/xgfone/go-tools/types"
)

func init() {
	RegisterStore("mysql", NewSQLStore("mysql"))
}

// sqlStore is the store backend based on SQL.
type sqlStore struct {
	driver string
	table  string
	engine *xorm.Engine
}

// NewSQLStore returns a new store backend based on SQL.
func NewSQLStore(driver string, table ...string) Store {
	tableName := "appconfig"
	if len(table) > 0 {
		tableName = table[0]
	}
	return &sqlStore{driver: driver, table: tableName}
}

func (s *sqlStore) Init(conf string) (err error) {
	var showSQL interface{}
	maxOpenConnNum := 0
	maxIdleConnNum := 0
	connTimeout := 3600

	vs := strings.Split(conf, "?")
	if len(vs) > 1 {
		ss := strings.Split(vs[1], "&")
		tmp := make([]string, 0, len(ss))
		for _, s := range ss {
			v := strings.Split(s, "=")
			if len(v) == 2 {
				switch v[0] {
				case "max_open_conn":
					if maxOpenConnNum, err = types.ToInt(v[1]); err != nil {
						return
					}
				case "max_idle_conn":
					if maxIdleConnNum, err = types.ToInt(v[1]); err != nil {
						return
					}
				case "timeout":
					if connTimeout, err = types.ToInt(v[1]); err != nil {
						return
					}
					tmp = append(tmp, s)
				case "show_sql":
					if showSQL, err = types.ToBool(v[1]); err != nil {
						return
					}
				default:
					tmp = append(tmp, s)
				}
			} else {
				tmp = append(tmp, s)
			}
		}

		conf = fmt.Sprintf("%s?%s", vs[0], strings.Join(tmp, "&"))
	}

	engine, err := xorm.NewEngine(s.driver, conf)
	if err != nil {
		return err
	}

	if maxOpenConnNum > 0 {
		engine.SetMaxOpenConns(maxOpenConnNum)
	}
	if maxIdleConnNum > 0 {
		engine.SetMaxIdleConns(maxIdleConnNum)
	}
	if connTimeout > 0 {
		engine.SetConnMaxLifetime(time.Duration(connTimeout) * time.Second)
	}
	if showSQL != nil {
		engine.ShowSQL(showSQL.(bool))
	}

	s.engine = engine
	return
}

// AppGetConfig is used by the app to get the value of the key in APP.
//
// If the time is 0 or negative, it should return the latest value.
// Or it should return the value at the provided time.
func (s *sqlStore) AppGetConfig(dc, env, app, key string, _time int64) (v string,
	err error) {

	session := s.engine.Select("`value`").Table(s.table)
	if _time > 0 {
		session = session.Where("`dc`=? AND `env`=? AND `app`=? AND `key`=? AND `time`=?",
			dc, env, app, key, _time)
	} else {
		session = session.Where("`dc`=? AND `env`=? AND `app`=? AND `key`=?",
			dc, env, app, key).Desc("`time`")
	}

	if ok, err := session.Get(&v); err != nil {
		return "", err
	} else if !ok {
		return "", ErrNotFound
	}

	return
}

// CreateDcAndEnv creates the new dc and env.
func (s *sqlStore) CreateDcAndEnv(dc, env string) error {
	sql := fmt.Sprintf("INSERT INTO `%s`(`dc`, `env`) VALUES (?, ?)", s.table)
	_, err := s.engine.Exec(sql, dc, env)
	return err
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
func (s *sqlStore) DeleteConfig(dc, env, app, key string, _time int64) error {
	args := make([]interface{}, 0, 5)
	where := "`dc`=?"
	args = append(args, dc)

	if env != "" {
		where += " AND `env`=?"
		args = append(args, env)

		if app != "" {
			where += " AND `app`=?"
			args = append(args, app)

			if key != "" {
				where += " AND `key`=?"
				args = append(args, key)

				if _time > 0 {
					where += " AND `time`=?"
					args = append(args, _time)
				}
			}
		}
	}

	sql := fmt.Sprintf("DELETE FROM `%s` WHERE %s", s.table, where)
	_, err := s.engine.Exec(sql, args...)
	return err
}

// GetAllDcAndEnvs returns all dc and env. The key is dc, and the value is
// the all envs in the dc.
func (s *sqlStore) GetAllDcAndEnvs() (map[string][]string, error) {
	rs, err := s.engine.Select("DISTINCT `dc`").Table(s.table).QueryString()
	if err != nil {
		return nil, err
	}

	if len(rs) == 0 {
		return map[string][]string{}, nil
	}

	result := make(map[string][]string, len(rs))
	for _, r := range rs {
		v, err := s.engine.Select("DISTINCT `env`").Table(s.table).Where(
			"`dc`=?", r["dc"]).QueryString()
		if err != nil {
			return nil, err
		}
		if len(v) == 0 {
			continue
		}

		envs := make([]string, len(v))
		for i, m := range v {
			envs[i] = m["env"]
		}
		result[r["dc"]] = envs
	}

	return result, nil
}

// SetKeyValue sets the key-value in dc, evn and app.
//
// If the key has not existed, it will create it; Or append it with a new
// timestamp.
func (s *sqlStore) SetKeyValue(dc, env, app, key, value string) error {
	sql := "INSERT INTO `%s`(`dc`, `env`, `app`, `key`, `time`, `value`) VALUES(?, ?, ?, ?, ?, ?)"
	sql = fmt.Sprintf(sql, s.table)
	_, err := s.engine.Exec(sql, dc, env, app, key, time.Now().Unix(), value)
	return err
}

// GetAllApps returns the names of all apps in dc and env.
//
// If search is not "", it will return those apps the name of which contains
// search.
//
// page is the ith page, and number the number of the apps in one page.
func (s *sqlStore) GetAllApps(dc, env, search string, page, number int64) (
	int64, []string, error) {

	where := "`dc`=? AND `env`=?"
	args := []interface{}{dc, env}

	if search != "" {
		where = fmt.Sprintf("%s AND `app` LIKE '%%%s%%'", where, search)
	}

	where += " GROUP BY `app`"

	if page > 0 && number > 0 {
		where = fmt.Sprintf("%s LIMIT %d OFFSET %d", where, number,
			(page-1)*number)
	}

	vm, err := s.engine.Select("count(1) AS count").Table(s.table).Where(where,
		args...).QueryInterface()
	if err != nil {
		return 0, nil, err
	}
	total := vm[0]["count"].(int64)
	if total < 1 {
		return 0, []string{}, nil
	}

	vs, err := s.engine.Select("DISTINCT `app`").Table(s.table).Where(where,
		args...).QueryString()
	if err != nil {
		return 0, nil, err
	}
	if len(vs) == 0 {
		return 0, []string{}, nil
	}
	apps := make([]string, len(vs))
	for i, m := range vs {
		apps[i] = m["app"]
	}

	return total, apps, nil
}

// GetAllKeys returns the names of all keys in dc, env and app.
//
// If search is not "", it will return those keys the name of which contains
// search.
//
// page is the ith page, and number the number of the apps in one page.
func (s *sqlStore) GetAllKeys(dc, env, app, search string, page,
	number int64) (int64, []string, error) {

	where := "`dc`=? AND `env`=? AND `app`=?"
	args := []interface{}{dc, env, app}

	if search != "" {
		where = fmt.Sprintf("%s AND `key` LIKE '%%%s%%'", where, search)
	}

	where += " GROUP BY `key`"

	if page > 0 && number > 0 {
		where = fmt.Sprintf("%s LIMIT %d OFFSET %d", where, number,
			(page-1)*number)
	}

	vm, err := s.engine.Select("count(1) AS count").Table(s.table).Where(where,
		args...).QueryInterface()
	if err != nil {
		return 0, nil, err
	}
	total := vm[0]["count"].(int64)
	if total < 1 {
		return 0, []string{}, nil
	}

	vs, err := s.engine.Select("DISTINCT `key`").Table(s.table).Where(where,
		args...).QueryString()
	if err != nil {
		return 0, nil, err
	}
	if len(vs) == 0 {
		return 0, []string{}, nil
	}
	keys := make([]string, len(vs))
	for i, m := range vs {
		keys[i] = m["key"]
	}

	return total, keys, nil
}

// GetAllValues returns the values of all keys in dc, env and app.
//
// page is the ith page, and number the number of the apps in one page.
//
// from and to is the start and end time to filte the values.
func (s *sqlStore) GetAllValues(dc, env, app, key string, page, number, from,
	to int64) (int64, map[int64]string, error) {

	where := "`dc`=? AND `env`=? AND `app`=? AND `key`=?"
	args := []interface{}{dc, env, app, key}

	if from > 0 {
		where = fmt.Sprintf("%s AND `time`>=%d", where, from)
	}
	if to > 0 {
		where = fmt.Sprintf("%s AND `time`<=%d", where, to)
	}

	if page > 0 && number > 0 {
		where = fmt.Sprintf("%s LIMIT %d OFFSET %d", where, number,
			(page-1)*number)
	}

	vm, err := s.engine.Select("count(1) AS count").Table(s.table).Where(where,
		args...).QueryInterface()
	if err != nil {
		return 0, nil, err
	}
	total := vm[0]["count"].(int64)
	if total < 1 {
		return 0, map[int64]string{}, nil
	}

	vs, err := s.engine.Select("`time`, `value`").Table(s.table).Where(where,
		args...).QueryInterface()
	if err != nil {
		return 0, nil, err
	}
	if len(vs) == 0 {
		return 0, map[int64]string{}, nil
	}
	values := make(map[int64]string, len(vs))
	for _, m := range vs {
		values[m["time"].(int64)] = string(m["value"].([]byte))
	}

	return total, values, nil
}
