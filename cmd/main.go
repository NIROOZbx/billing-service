package main

import (
	"log"

	"github.com/NIROOZbx/billing-service/config"
	"github.com/NIROOZbx/billing-service/internal/app"
)

func main() {
    
    cfg, err := config.LoadConfig()
    if err != nil {
        log.Fatalf("cannot load config: %v", err)
    }


    application, err := app.StartApp(cfg)
    if err != nil {
        log.Fatalf("cannot start app: %v", err)
    }
    defer application.Close()


    addr := ":" + cfg.App.Port
    if err := Run(application, addr); err != nil {
        log.Fatalf("cannot run program: %v", err)
    }

   
}