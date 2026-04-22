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
	"strings"
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

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
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
		} else {
			// Clean the path first, then remove leading slash to make it relative
			filename = path.Clean(filename)
			filename = strings.TrimPrefix(filename, "/")
		}
		slog.Info("Loading markdown", slog.String("filename", filename))

		md, err := root.ReadFile(filename)
		if err != nil {
			if os.IsNotExist(err) {
				slog.Info("File not found", slog.String("filename", filename))
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusNotFound)
				_, writeErr := w.Write([]byte("File not found"))
				if writeErr != nil {
					slog.Error("Cannot write 404 response", slog.Any("error", writeErr))
				}
			} else {
				slog.Error("Cannot read file", slog.String("filename", filename), slog.Any("error", err))
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)
				_, writeErr := w.Write([]byte("Internal server error"))
				if writeErr != nil {
					slog.Error("Cannot write 500 response", slog.Any("error", writeErr))
				}
			}
			return
		}

		html := mdToHTML(md)

		// Set proper headers before writing response
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300") // 5 minute cache
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

	// Apply security headers middleware
	secureHandler := securityHeaders(http.HandlerFunc(handler))
	http.Handle("/", secureHandler)

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
