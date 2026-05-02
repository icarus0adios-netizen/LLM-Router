package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "An LLM inference gateway with multi-dimensional scoring-based routing, SSE streaming proxy, and backpressure control.")
	})
	http.ListenAndServe(":8080", nil)

}
