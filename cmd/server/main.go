package main

import (
	"campusreturn/internal/app"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = ".runtime/data"
	}
	s, e := app.OpenStore(dir, os.Getenv("APP_ENCRYPTION_KEY"))
	if e != nil {
		log.Fatal(e)
	}
	defer s.DB.Close()
	var chain app.Chain
	if os.Getenv("APP_MODE") == "fabric" {
		chain, e = app.NewFabricChain()
		if e != nil {
			log.Fatal(e)
		}
	} else {
		chain = &app.DemoChain{S: s}
	}
	defer chain.Close()
	server, e := app.New(s, chain, dir, os.Getenv("ADMIN_PASSWORD"))
	if e != nil {
		log.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	server.Run(ctx)
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8088"
	}
	log.Printf("campus-return mode=%s addr=%s", chain.Mode(), addr)
	httpServer := &http.Server{Addr: addr, Handler: server.Router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		_ = httpServer.Shutdown(shutdown)
	}()
	if e = httpServer.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
