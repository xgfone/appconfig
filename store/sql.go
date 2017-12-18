package store

import (
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"github.com/xgfone/go-tools/types"
	"github.com/xgfone/log"
)

func init() {
	RegisterStore("mysql", NewSQLStore("mysql"))
}

// sqlStore is the store backend based on SQL.
type sqlStore struct {
	driver  string
	table   string
	cbtable string
	crtable string
	engine  *xorm.Engine
}

// NewSQLStore returns a new store backend based on SQL.
func NewSQLStore(driver string, table ...string) Store {
	tableName := "appconfig"
	cbtableName := "appcallback"
	cbResultTableName := "appresult"
	if len(table) == 1 {
		tableName = table[0]
	} else if len(table) == 2 {
		tableName = table[0]
		cbtableName = table[1]
	} else if len(table) > 2 {
		tableName = table[0]
		cbtableName = table[1]
		cbResultTableName = table[2]
	}
	return &sqlStore{
		driver:  driver,
		table:   tableName,
		cbtable: cbtableName,
		crtable: cbResultTableName,
	}
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

	engine.SetLogger(xorm.NewSimpleLogger(log.Std.Writer()))

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
func (s *sqlStore) AppGetConfig(dc, env, app, key string, _time int64) (
	v string, err error) {

	session := s.engine.Select("`value`").Table(s.table)
	if _time > 0 {
		where := "`dc`=? AND `env`=? AND `app`=? AND `key`=? AND `time`=?"
		session = session.Where(where, dc, env, app, key, _time)
	} else {
		where := "`dc`=? AND `env`=? AND `app`=? AND `key`=?"
		session = session.Where(where, dc, env, app, key).Desc("`time`")
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
	v, err := s.engine.Select("`id`").Table(s.table).Where("`dc`=? AND `env`=?",
		dc, env).Limit(1).QueryString()
	if err != nil {
		return err
	}
	if len(v) > 0 {
		return nil
	}

	sql := fmt.Sprintf("INSERT INTO `%s`(`dc`, `env`) VALUES (?, ?)", s.table)
	_, err = s.engine.Exec(sql, dc, env)
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

// SetKeyValue sets the key-value in dc, evn and app with a new timestamp.
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
	values := make(map[int64]string, len(vs))
	for _, m := range vs {
		values[m["time"].(int64)] = string(m["value"].([]byte))
	}

	return total, values, nil
}

func (s *sqlStore) AddCallback(dc, env, app, key, id, callback string) error {
	q := "INSERT INTO `%s`(`dc`,`env`,`app`,`key`,`cbid`,`callback`)VALUES(?,?,?,?,?,?)"
	sql := fmt.Sprintf(q, s.cbtable)
	_, err := s.engine.Exec(sql, dc, env, app, key, id, callback)
	return err
}

func (s *sqlStore) GetCallback(dc, env, app, key string) (map[string]string,
	error) {

	where := "`dc`=? AND `env`=? AND `app`=? AND `key`=?"
	vs, err := s.engine.Select("`cbid`, `callback`").Table(s.cbtable).Where(
		where, dc, env, app, key).QueryString()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(vs))
	for _, v := range vs {
		result[v["cbid"]] = v["callback"]
	}
	return result, nil
}

func (s *sqlStore) DeleteCallback(dc, env, app, key, id string) error {
	where := "`dc`=? AND `env`=? AND `app`=? AND `key`=?"
	args := []interface{}{dc, env, app, key}
	if id != "" {
		where += " AND `cbid`=?"
		args = append(args, id)
	}
	sql := fmt.Sprintf("DELETE FROM `%s` WHERE %s", s.cbtable, where)
	_, err := s.engine.Exec(sql, args...)
	return err
}

func (s *sqlStore) AddCallbackResult(dc, env, app, key, id, cb, r string) error {
	q := "INSERT INTO `%s`(`dc`,`env`,`app`,`key`,`cbid`,`callback`,`result`,`time`) VALUES(?,?,?,?,?,?,?,?)"
	sql := fmt.Sprintf(q, s.crtable)
	_, err := s.engine.Exec(sql, dc, env, app, key, id, cb, r, time.Now().Unix())
	return err
}

func (s *sqlStore) GetCallbackResult(dc, env, app, key, id string) (
	[][3]string, error) {

	where := "`dc`=? AND `env`=? AND `app`=? AND `key`=? AND `cbid`=?"
	vs, err := s.engine.Select("`callback`, `result`, `time`").Table(
		s.crtable).Where(where, dc, env, app, key, id).Desc("`time`").Limit(
		20, 0).QueryInterface()
	if err != nil {
		return nil, err
	}

	result := make([][3]string, len(vs))
	for i, v := range vs {
		result[i] = [3]string{fmt.Sprintf("%d", v["time"]),
			v["callback"].(string), v["result"].(string)}
	}
	return result, nil
}
