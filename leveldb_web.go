package leveldb_web

import (
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
)

var dbs sync.Map
var hostIp string
var debug = false

const prefix = "/leveldb_web/"
const testUrl = "/test_leveldb_web"

func getAddress() string {
	if envAddr := os.Getenv("LEVEL_WEB_ADDRESS"); envAddr != "" {
		return envAddr
	}

	return ":0"
}

func logInfo(str string) {
	if debug {
		log.Println(str)
	}
}

func logInfoWithFunc(c func()) {
	if debug {
		c()
	}
}

func Register(db *leveldb.DB, key string) {
	logInfo(fmt.Sprintf("add db register: %s, %p", key, db))

	dbs.Store(key, db)
}

func init() {
	if envAddr := os.Getenv("LEVEL_WEB_DEBUG"); envAddr == "true" {
		debug = true
	}

	go RunWebServer()
}

func RunWebServer() error {
	listen, err := net.Listen("tcp", getAddress())

	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc(testUrl, func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("hello world"))
	})

	mux.HandleFunc(prefix, DBList)

	port := listen.Addr().(*net.TCPAddr).Port

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	hostIp = fmt.Sprintf("http://127.0.0.1:%d", port)

	log.Printf("leveldb web server on: %s", hostIp)

	return server.Serve(listen)
}

func DBList(writer http.ResponseWriter, request *http.Request) {
	reg := regexp.MustCompile(prefix + `(([\w]*))(\/(.*))?`)

	logInfoWithFunc(func() {
		var dbList []string
		dbs.Range(func(key, value interface{}) bool {
			dbList = append(dbList, fmt.Sprintf("%v", key))
			return true
		})

		logInfo(fmt.Sprintf("path: %s, %v", request.URL.Path, dbList))
	})

	if reg.MatchString(request.URL.Path) {
		base := reg.FindSubmatch([]byte(request.URL.Path))

		var res [][]byte

		for _, s := range base {
			if string(s[:]) != "" {
				res = append(res, s)
			}
		}

		if len(res) <= 2 {
			var dbList []string
			dbs.Range(func(key, value interface{}) bool {
				p := fmt.Sprintf(`<p><a href="%s%v">%v</a></p>`, prefix, key, key)
				dbList = append(dbList, p)
				return true
			})
			var html string

			for _, p := range dbList {
				html += fmt.Sprintf("\n%s", p)
			}

			writer.Write([]byte(html))
			return
		}

		if len(res) == 3 {
			db := string(res[2])
			KeyList(db)(writer, request)
			return
		}

		if len(res) == 5 {
			db := string(res[2])
			recordKey := string(res[4])
			ValueList(db, recordKey)(writer, request)
		}

	} else {
		http.NotFound(writer, request)
	}
}

func KeyList(dbKey string) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		var keys []string
		if load, ok := dbs.Load(dbKey); ok {
			db := load.(*leveldb.DB)

			iter := db.NewIterator(util.BytesPrefix([]byte("")), nil)
			defer iter.Release()

			for iter.Next() {
				p := fmt.Sprintf(`<p><a href="%s%s/%s">%s</a></p>`, prefix, dbKey, string(iter.Key()[:]), string(iter.Key()[:]))
				keys = append(keys, p)
			}

			err := iter.Error()
			if err != nil {
				http.NotFound(writer, request)
			}

			var html string

			for _, p := range keys {
				html += fmt.Sprintf("\n%s", p)
			}

			writer.Write([]byte(html))
		} else {
			http.NotFound(writer, request)
		}
	}
}

func ValueList(dbKey string, recordKey string) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		if load, ok := dbs.Load(dbKey); ok {
			db := load.(*leveldb.DB)

			if has, err := db.Has([]byte(recordKey), nil); has && err == nil {
				get, err := db.Get([]byte(recordKey), nil)
				if err != nil {
					http.NotFound(writer, request)
				}
				writer.Write(get)
			} else {
				http.NotFound(writer, request)
			}

		} else {
			http.NotFound(writer, request)
		}
	}
}
