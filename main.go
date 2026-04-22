package main

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"golang.org/x/sync/errgroup"

	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

type Config struct {
	DefaultIndex    string
	Dir             string
	Port            string
	ShutdownTimeout time.Duration
}

func mdToHTML(md []byte) []byte {
	// create markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return markdown.Render(doc, renderer)
}

func newConfig() *Config {
	config := &Config{}
	flag.StringVar(&config.Dir, "d", ".", "Markdown directory")
	flag.StringVar(&config.Port, "p", ":8080", "HTTP port (default: 8080)")
	flag.Parse()
	config.DefaultIndex = "README.md"
	config.ShutdownTimeout = 1 * time.Second
	return config
}

func newServeHTTP(config *Config) (http.HandlerFunc, error) {
	root, err := os.OpenRoot(config.Dir)
	if err != nil {
		panic(err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Path
		if r.URL.Path == "/" {
			filename = config.DefaultIndex
		}
		filename = path.Clean(filename)
		slog.Info("Loading markdown", slog.String("filename", filename))

		md, err := root.ReadFile(filename)
		if err != nil {
			slog.Error("Cannot read file", slog.String("filename", filename), slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		html := mdToHTML(md)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(html)
		if err != nil {
			slog.Error("Cannot write response", slog.Any("error", err))
		}
	}, nil
}

func main() {
	config := newConfig()

	handler, err := newServeHTTP(config)
	if err != nil {
		slog.Error("Cannot instantiate HTTP handler", slog.Any("error", err))
		os.Exit(1)
	}
	http.HandleFunc("/", handler)

	server := &http.Server{
		Addr:              config.Port,
		Handler:           nil,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Setup signals trap
	sigChan, sigCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	g, gCtx := errgroup.WithContext(sigChan)

	// Listen and serve in a Goroutine to support gracefull shutdown
	g.Go(func() error {
		slog.Info("Listening...", slog.String("Addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// Try graceful shutdown once errGroup is closed after signals were received
	g.Go(func() error {
		<-gCtx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(c)
	})

	if err := g.Wait(); err != nil {
		sigCancel()
		slog.Error(err.Error(), slog.Any("error", err))
		os.Exit(1)
	}

	sigCancel()
}
