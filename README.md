# appconfig

`appconfig` is a centralized configuration manager of the APP.


## Goal

- Simple RESTfull API.
- App uses the key-value configuration. And support many keys for an app.
- No dependency, but the backend store, such as `ZooKeeper` or `MySQL`.
- Notify the APP asynchronously when the watched configuration has been changed.
- Support many Data-Center and many Environment.
- Simple deployment, only a binary program.
- An user-friendly Web manager interface.


## Todo List

- [x] RESTfull API.
- [x] Many Data-Center(`dc`) and many Environment(`env`).
- [x] Notify the changed configuration to the watching app asynchronously by the callback way with `HTTP`.
- [x] Backend store `Memory` implementation, which is only used to test.
- [x] Backend store `ZooKeeper` implementation.
- [x] Backend store `MySQL` implementation.
- [ ] Backend store `Redis` implementation.
- [ ] Backend store `Etcd` implementation.
- [ ] Web manager interface.
- [ ] Authentication and Authorization.


## Run

### Install
```bash
$ go get github.com/xgfone/appconfig
$ cd $GOPATH/src/github.com/xgfone/appconfig
$ dep ensure
$ go install github.com/xgfone/appconfig
```

### Usage
```bash
$ appconfig -h

Usage of ./appconfig:
  -addr string
        The address to listen to. (default ":80")
  -conf string
        The configration information of the backend store.
  -logfile string
        the log file path.
  -loglevel string
        the log level, such as DEBUG, INFO, etc. (default "DEBUG")
  -store string
        The backend store type, such as memory, zk, or mysql (default "memory")
  -version
        Print the version and exit.
```

**Notice**: For HA and LB, you can run many instances, only if they use the same backend store.


### Use `Memory` as Backend Store
```bash
$ appconfig
```

`Memory` does not need the config option `conf`.

### Use `ZooKeeper` as Backend Store
```bash
$ appconfig -store zk -conf "addr=10.241.230.105,10.241.230.106,10.241.230.107&root=/config"
```

For `ZooKeeper` backend store, the config option `store` and `conf` must be given. The value of `store` must be `zk`, and `conf` is ZooKeeper configuration, which uses the format `application/x-www-form-urlencoded`, and supports three options:

1. **`addr`**: The address list of the ZooKeeper cluster, which are separated by the comma. The port may be omitted, which is 2181 by default.
2. **`root`**: The path prefix used by the configuration. The default is "/".
3. **`timeout`**: The timeout to connect to ZooKeeper, the unit of which is second. The default is 3.


Notice:

- If there is no any option name to be specified, it is the addess list by default, such as `-conf "10.241.230.105,10.241.230.106,10.241.230.107"` is equal to `-conf "addr=10.241.230.105,10.241.230.106,10.241.230.107"`.
- The ZooKeeper implementation uses the sub-directories: `config` for the key-value configuration of the app, `callback` for the callback information of the configuration, `cbresult` for the result of the callback. **This implementation will create the sub-directories automatically when the program starts. If failed to create them, the program exits and prints the error.**


### Use `MySQL` as Backend Store
```bash
$ appconfig -store mysql -conf "user:password@tcp(host:port)/db?show_sql=1&timeout=300"
```

For `MySQL` backend store, the config option `store` and `conf` must be given. The value of `store` must be `mysql`, and `conf` is MySQL configuration, which uses the format [DSN](https://github.com/go-sql-driver/mysql#dsn-data-source-name), because the manager uses MySQL driver `github.com/go-sql-driver/mysql`.

On the basis of `DSN`, the manager adds three new options:

1. **`max_open_conn`**: The maximum number of open connections to MySQL. The default is `0` (unlimited).
2. **`max_idle_conn`**: The maximum number of connections in the idle connection pool. The default is `2`.
3. **`show_sql`**: It's a bool. If true, it will print the executed RAW SQL. The default is false. For `t`, `T`, `1`, `true`, `True`, `TRUE`, it's true. For `f`, `F`, `0`, `false`, `False`, `FALSE`, it's false.

Notice:

- If the MySQL server has set the idle timeout of the client connection, suggest to add the option `timeout`, and its value should be less than the server setting value.
- The MySQL implementation uses three tables: `appconfig` for the key-value configuration of the app, `appcallback` for the callback information of the configuration, `appresult` for the result of the callback.
- You should create the three tables before running the program. For the SQL model, refer to [here](https://github.com/xgfone/appconfig/blob/master/docs/model.sql).


## V1 API

The current api is `v1`. The api below is under the prefix `/v1`, such as `/v1/app/{dc}/{env}/{app}/{key}` for app to get the configuration information.

For `APP`, it should only use three apis:

1. Get the configuration of a key. ([API 1.](https://github.com/xgfone/appconfig#1-app-get-the-configuration-of-a-key))
2. Register a callback to watch the change of the configuration of a key. ([API 13.](https://github.com/xgfone/appconfig#13-add-the-callback-to-watch-a-certain-key))
3. Delete the callbacks registered by it. ([API 14.](https://github.com/xgfone/appconfig#14-delete-the-callback-of-a-certain-key))

**Suggest:** If the app want to watch the change of the configuration of a key, it maybe register a callback for it when app starts, and delete the callback before the app exits.

**Notice:** `dc`, `env`, `app`, `key` should be a plain string, which must not contain any character in `/&=?%#+\`. In general, it should only use `a-z`, `A-Z`, `0-9` and `-_`. Certainly, they are not mandatory, you can use other characters, such as `@$^:`. Also, you maybe use Chinese. For the length limit, it's no limit in principle, which depends on the backend store. For example, the default SQL model, the maximum length of `dc`, `env`, `app` is `32`, the maximum length of `key` and `id` of the callback is `64`, the maximum length of `callback` and `result` of the callback is `256`.


### 1. App Get the Configuration of a Key

#### Request
`GET /app/{dc}/{env}/{app}/{key}[?time=unixstamp]`

If giving the `time` query option, only return the configuration value at the specified time. You maybe consider it as the verison. If not giving, only return the lastest configuration value.

Notice: when changing the configuration of a certain key, the old one won't be deleted or overrided, which is just saved as the snapshot in order to recover or reuse.

#### Response
Body is the configuration info, which is parsed by the app, and the configuration manager does not care about its format.

Notice: If there is not the key, return `404`.


### 2. Admin Create DC and Env

#### Request
`POST /admin?dc={dc}&env={env}`

Create an environment named `env` in the data center named `dc`. If `dc` does not exist, create it firstly. If `dc` and `env` has existed, do nothing.

#### Response
None.


### 3. Admin Get All DC and Env

#### Request
`GET /admin`

#### Response
Body is `JSON` string, the key of which is the DC name, and the value of that is a string array that are all the Environment names in `DC`. For example,
```json
{
    "beijing": ["production", "dev"],
    "shanghai": ["test", "dev"]
}
```


### 4. Admin Upload the Key-Value Configuration

#### Request
`POST /admin/{dc}/{env}/{app}/{key}`

Notice: Body is the value of the key.

#### Response
None.

Notice: When uploading the configuration value of a key, it will get all the callbacks of this key, and notify the changed value to the corresponding app asynchronously.


### 5. Admin Get All Apps in DC and Env

#### Request
`GET /admin/{dc}/{env}[?page={page}&size={size}&search={search}]`

Each of the query `page`, `size` and `search` can be ignored. The interface uses the pagination function. `page` is the page number, which is `1` by default. `size` is the size of one page, that's, how many items a page has, which is `20` by default. `search` is used to filte the apps by its name.

#### Response

Body is `JSON` string. For example, `GET /admin/beijing/dev?page=2`

```json
{
    "total": 22, // The total number of all the apps.
    "apps": ["app21", "app22"]
}
```


### 6. Admin Get All Keys of App in DC and Env

#### Request
`GET /admin/{dc}/{env}/{app}[?page={page}&size={size}&search={search}]`

Each of the query `page`, `size` and `search` can be ignored. The interface uses the pagination function. `page` is the page number, which is `1` by default. `size` is the size of one page, that's, how many items a page has, which is `20` by default. `search` is used to filte the keys by its name.

#### Response

Body is `JSON` string. For example, `GET /admin/beijing/dev/app1?page=2`

```json
{
    "total": 22, // The total number of all the keys.
    "keys": ["key21", "key22"]
}
```


### 7. Admin Get All Values of the Specified Key

#### Request
`GET /admin/{dc}/{env}/{app}/{key}[?page={page}&size={size}&from={unixstamp}&to={unixstamp}]`

Each of the query `page`, `size`, `from` and `to` can be ignored. The interface uses the pagination function. `page` is the page number, which is `1` by default. `size` is the size of one page, that's, how many items a page has, which is `20` by default. `from` and `to` are used to filte the values between them. `from` is `0` by default, which starts with the first value, and `to` is `0` by default, which ends with the last value.

#### Response

Body is `JSON` string. For example, `GET /admin/beijing/dev/app1/key2?page=2`

```json
{
    "total": 22, // The total number of all the values.
    "values": {
        "1513489741": "value21",
        "1513489742": "value22"
    }
}
```

Notice: the value of `values` is `JSON`, the key of which is the unixstamp, and the value of that is the corresponding value.


### 8. Admin Delete the Whole DC

#### Request
`DELETE /admin/{dc}`

#### Response
None.

Notice: If the `dc` does not exist, do nothing.


### 9. Admin Delete the Whole Env in DC

#### Request
`DELETE /admin/{dc}/{env}`

#### Response
None.

Notice: If the `env` does not exist, do nothing.


### 10. Admin Delete the Whole App in DC and Env

#### Request
`DELETE /admin/{dc}/{env}/{app}`

#### Response
None.

Notice: If the `app` does not exist, do nothing.


### 11. Admin Delete the Whole Key of an App in DC and Env

#### Request
`DELETE /admin/{dc}/{env}/{app}/{key}[?time={unixstamp}]`

If giving the query argument `time`, only delete the value of the specified time. Or delete all the values.

#### Response
None.

Notice: If the specified `key` does not exist, do nothing.


### 12. Get All the Callbacks of a Certain Key

#### Request
`GET /callback/{dc}/{env}/{app}/{key}`

#### Response

Body is a `JSON` string, the key of which is the id of the registered callback, and the value of that is the callback value.


### 13. Add the Callback to Watch a Certain Key

#### Request
`POST /callback/{dc}/{env}/{app}/{key}/{id}`

`id` is the identifier of the callback, which should be unique under `/{dc}/{env}/{app}/{key}`. The limit is the same as `dc`, `env`, `app` or `key`. For a key, there may be many apps to watch it. So the `id` is used to distinguish them.

Notice: Body is the callback value. The current version only supports the HTTP URL by `POST`, that's, the callback should be a HTTP URL that supports the request method `POST`.

When invoking the callback by HTTP POST, The body is the changed new value. If you have many callbacks and want to distinguish them, you need to add `key` or `id` to the URL.

#### Response
None.


### 14. Delete the Callback of a Certain Key

#### Request
`DELETE /callback/{dc}/{env}/{app}/{key}[?id={id}]`

`id` is the identifier of the callback. If giving the query argument `id`, only delete the callback specified by the id. Or delete all the callbacks.

#### Response
None.

If the callback does not exist, do nothing.


### 15. Get the Result of the Callback Notification

#### Request
`GET /callback/{dc}/{env}/{app}/{key}/{id}`

`id` is the identifier of the callback.

#### Response

Body is a `JSON` string, which only has key, `result`. Its value is an array, each element of which is three-tuples. The first is the unixstamp time, the second is the callback address, and the second is the result. If the callback is successful, the result is "". Or it is the error string. For example,

```json
{
    "result": [
        ["1513489740", "http://127.0.0.1:8000/callback/path", ""],               // Success
        ["1513489750", "http://127.0.0.1:9000/callback/path", "cannot connect"], // Failure
        // ......
    ]
}
```
