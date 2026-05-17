package log

import (
	"io"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"golang.org/x/term"
)

type Options struct {
	Verbose bool
	NoColor bool
	Writer  io.Writer
}

func Setup(opts Options) {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}

	noColor := opts.NoColor || os.Getenv("NO_COLOR") != "" || !isTerminal(w)
	level := slog.LevelInfo
	if opts.Verbose {
		level = slog.LevelDebug
	}

	handler := tint.NewHandler(w, &tint.Options{
		Level:   level,
		NoColor: noColor,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})
	slog.SetDefault(slog.New(handler))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
