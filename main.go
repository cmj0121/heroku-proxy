package main

import (
	"fmt"
	"strconv"
	"os"
	"os/signal"
	"syscall"
	"net/http"
	"context"

	"github.com/cmj0121/logger"
	"github.com/cmj0121/structopt"
)

const (
	// The environment of the bind PORT
	ENV_PORT = "PORT"
)

type Server struct {
	structopt.Help

	Port int `help:"The bind port"`

	*logger.Log `-` // nolint
	LogLevel    string `name:"log" short:"l" choice:"warn info debug trace" help:"set the log level"`
}

func (serv *Server) Run() {
	mux := http.NewServeMux()
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
	w.Write([]byte("Hello, heroku-proxy")) // nolint
}

func main() {
	// Get the bind port from the environment
	serv := &Server{
		Port: 8000,
		Log: logger.New("heroku-proxy"),
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
	parser := structopt.MustNew(serv)
	parser.Run()
	serv.Log.Level = logger.LogLevel(serv.LogLevel)
	// run the HTTP server
	serv.Run()
}
