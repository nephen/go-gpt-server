package api

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func HandleSSE(w http.ResponseWriter, r *http.Request) {
	appId := r.URL.Query()["appId"]
	page := r.URL.Query()["page"]
	pageSize := r.URL.Query()["pageSize"]

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Panic("server not support")
	}
	for i := 0; i < 2; i++ {
		time.Sleep(2 * time.Second)
		fmt.Fprintf(w, "data: %d%s%s%s\n\n", i, appId[0], page[0], pageSize[0])
		flusher.Flush()
	}
	fmt.Fprintf(w, "event: close\ndata: close\n\n") // 一定要带上data，否则无效
}
