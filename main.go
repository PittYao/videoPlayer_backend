package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"videoPlayer/config"
	valid "videoPlayer/validator"
	"videoPlayer/web"
)

func main() {
	valid.InitValidator() // 字段验证

	go web.ServeHTTP() // 路由
	go config.ServeStreams()

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println(sig)
		done <- true
	}()
	log.Println("Server Start Awaiting Signal")
	<-done
	log.Println("Exiting")
}
