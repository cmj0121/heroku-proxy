package main

import (
	"io"
	"fmt"
	"strconv"
	"os"
	"os/signal"
	"syscall"
	"net/http"
	"net/url"
	"context"

	"github.com/cmj0121/logger"
)

const (
	// The environment of the bind PORT
	ENV_PORT = "PORT"
	// The exposed query key
	KEY_QUERY_API = "q"
)

type Server struct {
	Port int `help:"The bind port"`

	*logger.Log `-` // nolint
	LogLevel    string `name:"log" short:"l" choice:"warn info debug trace" help:"set the log level"`

	*http.Client
}

func (serv *Server) Run() {
	mux := http.NewServeMux()
	mux.HandleFunc("/proxy", serv.Proxy)
	mux.Handle("/", serv)

	srv := http.Server{
		Addr: fmt.Sprintf(":%v", serv.Port),
		Handler: mux,
	}
	// make sure the gracefully shutdown goroutine will be closed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// gracefully shut-down the server
	conn := make(chan struct{})
	go func(ctx context.Context) {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)

		select {
		case <-sigint:
			// wait until receive the SIGINT
		case <-ctx.Done():
			// function exist
		}

		// receive the signal, shutdown the server
		if err := srv.Shutdown(context.Background()); err != nil {
			// cannot close the server, may timeout
			serv.Warn("HTTP server gracefully shutdown: %v", err)
		}

		serv.Info("HTTP server success gracefully shutdown")
		close(conn)
	}(ctx)

	// start run the HTTP server
	serv.Info("HTTP server start and bind on :%v", serv.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		serv.Crit("HTTP server: %v", err)
		return
	}

	cancel()
	<-conn
	serv.Info("HTTP server closed")
}

// default HTTP handler
func (serv *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("Hello, heroku-proxy\n")) // nolint
}

// the proxy service
func (serv *Server) Proxy(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(fmt.Sprintf("error: %v\n", r))) // nolint
		}
	}()
	method := r.Method
	query_url := r.URL.Query().Get(KEY_QUERY_API)

	switch {
	case query_url == "":
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("?%v=YOUR_TARGET_ENDPOINT\n", KEY_QUERY_API))) // nolint
	default:
		endpoint_url, err := url.Parse(query_url)
		if err != nil {
			serv.Warn("invalid url %#v: %v", query_url, err)
			err = fmt.Errorf("invalid url %#v: %v", query_url, err)
			panic(err)
		}

		req, err := http.NewRequest(method, endpoint_url.String(), r.Body)
		if err != nil {
			serv.Warn("invalid request %v", err)
			err = fmt.Errorf("invalid request %v", err)
			panic(err)
		}
		defer r.Body.Close()

		// override the user agent
		req.Header.Set("User-Agent", r.UserAgent())

		// send the request
		res, err := serv.Client.Do(req)
		if err != nil {
			serv.Warn("cannot send request: %v", err)
			err = fmt.Errorf("cannot send request: %v", err)
			panic(err)
		}
		// override the status code and body
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body) // nolint
	}
}

func main() {
	// Get the bind port from the environment
	serv := &Server{
		Port: 8000,
		Log: logger.New("heroku-proxy"),
		Client: &http.Client{},
	}
	port := os.Getenv(ENV_PORT)
	if port != "" {
		p, err := strconv.Atoi(port)
		if err != nil || p <= 0 {
			// cannot set the port from ENV
			os.Stderr.WriteString(fmt.Sprintf("invalid port %#v, override as %v\n", port, serv.Port))
		}
		serv.Port = p
	}

	// start parse from command-line
	// parser := structopt.MustNew(serv)
	// parser.Run()
	serv.Log.Level = logger.INFO
	// run the HTTP server
	serv.Run()
}
