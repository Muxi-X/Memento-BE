package main

import (
	"context"
	"log"

	"cixing/internal/app/api"
)

// 只负责启动 api
func main() {
	if err := api.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
