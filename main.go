// Command cmdserver is a simple web server that runs an external command on each request,
// and responds with the output of such command.
//
// Usage example:
//
//	cmdserver tail file1.log
package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/artyom/httpgzip"
)

func main() {
	log.SetFlags(0)
	addr := "localhost:8080"
	var reload int
	flag.StringVar(&addr, "addr", addr, "address to listen")
	flag.IntVar(&reload, "r", reload, "reload page every N `seconds`")
	flag.Parse()
	if err := run(addr, reload, flag.Args()); err != nil {
		log.Fatal(err)
	}
}

func run(addr string, reload int, cmdargs []string) error {
	if len(cmdargs) == 0 {
		return errors.New("need command and its arguments to call")
	}
	mux := http.NewServeMux()
	mux.Handle("GET /", httpgzip.New(&handler{cmdargs: cmdargs, reload: reload, sema: make(chan struct{}, 1)}))
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  time.Second,
		WriteTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

type handler struct {
	cmdargs []string
	reload  int
	sema    chan struct{}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case h.sema <- struct{}{}:
		defer func() { <-h.sema }()
	case <-r.Context().Done():
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	if h.reload > 0 {
		w.Header().Set("Refresh", strconv.Itoa(h.reload))
	}
	w.Header().Set("Cache-Control", "no-store")
	cmd := exec.CommandContext(r.Context(), h.cmdargs[0], h.cmdargs[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) == 0 {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write(out)
		}
		return
	}
	w.Write(out)
}
