// task275-inkorder 法庭文书笔迹笔顺证据复核台
//
// 服务入口：--addr 监听地址，--db SQLite 路径，--smoke-test 自检模式。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"task275-inkorder/internal/httpapi"
	"task275-inkorder/internal/service"
	"task275-inkorder/internal/smoke"
	"task275-inkorder/internal/store"
)

func main() {
	var (
		addr      = flag.String("addr", ":8080", "HTTP 监听地址")
		dbPath    = flag.String("db", "task275-inkorder.db", "SQLite 数据库路径")
		smokeTest = flag.Bool("smoke-test", false, "运行自检后退出")
	)
	flag.Parse()

	if *smokeTest {
		if err := smoke.Run(*dbPath); err != nil {
			log.Fatalf("smoke test failed: %v", err)
		}
		fmt.Println("smoke test passed")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer db.Close()

	app := service.NewApp(db)
	h := httpapi.NewHandler(app)
	log.Printf("task275-inkorder listening on %s (db=%s)", *addr, *dbPath)
	log.Fatal(http.ListenAndServe(*addr, h.Router()))
}
