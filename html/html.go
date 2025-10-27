package html

import (
	"html/template"
	"io"

	"github.com/fyrna/mofu"
)

type Config struct {
	Funcs       template.FuncMap
	Delimiters  []string // [left, right]
	Development bool
}

type HTMLTemplate struct {
	t      *template.Template
	config *Config
}

func Sparkle(pattern string, cfgs ...*Config) mofu.TemplateConfig {
	cfg := &Config{}

	if len(cfgs) > 0 && cfgs[0] != nil {
		cfg = cfgs[0]
	}

	return &htmlConfig{
		pattern: pattern,
		config:  cfg,
	}
}

type htmlConfig struct {
	pattern string
	config  *Config
}

func (h *htmlConfig) CreateEngine() (mofu.TemplateEngine, error) {
	t := template.New("")

	// Apply delimiters if provided
	if len(h.config.Delimiters) == 2 {
		t = t.Delims(h.config.Delimiters[0], h.config.Delimiters[1])
	}

	// Apply funcs if provided
	if h.config.Funcs != nil {
		t = t.Funcs(h.config.Funcs)
	}

	// Parse templates
	t, err := t.ParseGlob(h.pattern)
	if err != nil {
		return nil, err
	}

	return &HTMLTemplate{
		t:      t,
		config: h.config,
	}, nil
}

func (h *HTMLTemplate) Render(w io.Writer, name string, data any) error {
	return h.t.ExecuteTemplate(w, name, data)
}
