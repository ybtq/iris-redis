package redis

import (
	"runtime"
	"time"

	"github.com/kataras/golog"
	"github.com/kataras/iris/sessions"
	"github.com/ybtq/iris-redis/service"
)

// Database the redis back-end session database for the sessions.
type Database struct {
	redis *service.Service
}

var _ sessions.Database = (*Database)(nil)

// New returns a new redis database.
func New(cfg ...service.Config) *Database {
	db := &Database{redis: service.New(cfg...)}
	db.redis.Connect()
	_, err := db.redis.PingPong()
	if err != nil {
		golog.Debugf("error connecting to redis: %v", err)
		return nil
	}
	runtime.SetFinalizer(db, closeDB)
	return db
}

// Config returns the configuration for the redis server bridge, you can change them.
func (db *Database) Config() *service.Config {
	return db.redis.Config
}

// Acquire receives a session's lifetime from the database,
// if the return value is LifeTime{} then the session manager sets the life time based on the expiration duration lives in configuration.
func (db *Database) Acquire(sid string, expires time.Duration) sessions.LifeTime {
	seconds, hasExpiration, found := db.redis.TTL(sid)
	if !found {
		// not found, create an entry with ttl and return an empty lifetime, session manager will do its job.
		var emptyData []byte
		if err := db.redis.Set(sid, emptyData, int64(expires.Seconds())); err != nil {
			golog.Error(err)
		}

		return sessions.LifeTime{} // session manager will handle the rest.
	}

	if !hasExpiration {
		return sessions.LifeTime{}

	}
	return sessions.LifeTime{Time: time.Now().Add(time.Duration(seconds) * time.Second)}
}

// OnUpdateExpiration will re-set the database's session's entry ttl.
// https://redis.io/commands/expire#refreshing-expires
func (db *Database) OnUpdateExpiration(sid string, newExpires time.Duration) error {
	return db.redis.UpdateTTL(sid, int64(newExpires.Seconds()))
}

// Set sets a key value of a specific session.
// Ignore the "immutable".
func (db *Database) Set(sid string, lifetime sessions.LifeTime, key string, value interface{}, immutable bool) {
	store := NewStore()
	db.get(sid, store)
	store.values[key] = value
	golog.Debug("Set", sid, lifetime, key, value, store.values)
	db.set(sid, int64(lifetime.DurationUntilExpiration().Seconds()), store)
}

func (db *Database) set(sid string, secondsLifetime int64, store *Store) {
	valueBytes, err := store.Serialize()
	if err != nil {
		golog.Error(err)
		return
	}
	if err = db.redis.Set(sid, valueBytes, secondsLifetime); err != nil {
		golog.Error(err)
	}
}

// Get retrieves a session value based on the key.
func (db *Database) Get(sid string, key string) (value interface{}) {
	store := NewStore()
	db.get(sid, store)
	value = store.values[key]
	return
}

func (db *Database) get(key string, store *Store) {
	data, err := db.redis.Get(key)
	if err != nil {
		// not found.
		return
	}

	err = store.Deserialize(data.([]byte))
	if err != nil {
		golog.Error(err)
		return
	}
}

func (db *Database) keys(sid string) []string {
	var keys []string
	if db.redis.Exist(sid) {
		keys = append(keys, sid)
	}
	return keys
}

// Visit loops through all session keys and values.
func (db *Database) Visit(sid string, cb func(key string, value interface{})) {
	store := NewStore()
	db.get(sid, store)
	for key, value := range store.values {
		cb(key.(string), value)
	}
}

// Len returns the length of the session's entries (keys).
func (db *Database) Len(sid string) (n int) {
	store := NewStore()
	db.get(sid, store)
	return len(store.values)
}

// Delete removes a session key value based on its key.
func (db *Database) Delete(sid string, key string) (deleted bool) {
	store := NewStore()
	db.get(sid, store)
	_, ok := store.values[key]
	if ok {
		delete(store.values, key)
		db.set(sid, 0, store)
		deleted = true
	}
	return
}

// Clear removes all session key values but it keeps the session entry.
func (db *Database) Clear(sid string) {
	store := NewStore()
	db.get(sid, store)
	if len(store.values) > 0 {
		store.Flush()
		db.set(sid, 0, store)
	}
}

// Release destroys the session, it clears and removes the session entry,
// session manager will create a new session ID on the next request after this call.
func (db *Database) Release(sid string) {
	// clear all $sid-$key.
	db.Clear(sid)
	// and remove the $sid.
	db.redis.Delete(sid)
}

// Close terminates the redis connection.
func (db *Database) Close() error {
	return closeDB(db)
}

func closeDB(db *Database) error {
	return db.redis.CloseConnection()
}
