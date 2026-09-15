// FlowSight server: HTTP API + ingestion scheduler in one process.
// Offline without SECTORS_API_KEY (serves seed data); live with it.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"flowsight/internal/api"
	"flowsight/internal/config"
	"flowsight/internal/sectors"
	"flowsight/internal/store"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		log.Fatalf("server: mkdir data: %v", err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("server: open db: %v", err)
	}
	defer db.Close()
	cache := store.NewCache(cfg.RedisURL)
	sectorsClient := sectors.New(cfg.SectorsBaseURL, cfg.SectorsAPIKey)

	// Historical-replay seed keeps every screen alive with no key.
	if db.NeedsSeed() {
		seeded := false
		for _, dir := range []string{"tests/fixtures", "../tests/fixtures", "backend/tests/fixtures"} {
			st, err := db.SeedFromDir(dir, cfg.DemoUserKey)
			if err == nil {
				log.Printf("server: seeded %d snapshots from %s", st.Snapshots, dir)
				seeded = true
				break
			}
		}
		if !seeded {
			if ex, err := os.Executable(); err == nil {
				binDir := filepath.Dir(ex)
				for _, dir := range []string{
					filepath.Join(binDir, "fixtures"),
					filepath.Join(binDir, "..", "share", "flowsight"),
				} {
					if st, err := db.SeedFromDir(dir, cfg.DemoUserKey); err == nil {
						log.Printf("server: seeded %d snapshots from %s", st.Snapshots, dir)
						seeded = true
						break
					}
				}
				if !seeded {
					log.Printf("server: no seed bundle found (tried tests/fixtures, binary dir)")
				}
			}
		}
	}
	// Ensure the demo account covers the "semua" default: every ticker with
	// stored data (fallback: config watchlist).
	if wl, _ := db.Watchlist(cfg.DemoUserKey); len(wl) == 0 {
		seeds, _ := db.AllTickers()
		if len(seeds) == 0 {
			seeds = cfg.Watchlist
		}
		for _, t := range seeds {
			_ = db.AddWatch(cfg.DemoUserKey, t)
		}
	}

	srv := api.New(cfg, db, cache, sectorsClient)
	srv.Sched.Start()
	defer srv.Sched.Stop()

	httpSrv := &http.Server{Addr: ":" + cfg.Port, Handler: srv.Router()}
	go func() {
		log.Printf("server: listening :%s (sectors key: %v, redis: %v)",
			cfg.Port, cfg.HasSectorsKey(), cache.HasRedis())
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Print("server: stopped")
}
