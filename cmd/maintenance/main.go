package main

import (
	"campusreturn/internal/app"
	"fmt"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "backup" {
		log.Fatal("usage: maintenance backup OUTPUT.db")
	}
	s, e := app.OpenStore(os.Getenv("DATA_DIR"), os.Getenv("APP_ENCRYPTION_KEY"))
	if e != nil {
		log.Fatal(e)
	}
	defer s.DB.Close()
	if _, e = s.DB.Exec("VACUUM INTO ?", os.Args[2]); e != nil {
		log.Fatal(e)
	}
	if e = os.Chmod(os.Args[2], 0600); e != nil {
		log.Fatal(e)
	}
	fmt.Println("Consistent SQLite snapshot created")
}
