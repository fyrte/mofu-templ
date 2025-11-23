package html

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"maps"
	"path/filepath"
	"sync"
	"time"

	"github.com/fyrna/mofu"
)

type HTMLTemplate struct {
	t        *template.Template
	config   *Config
	pattern  string
	mu       sync.RWMutex
	lastLoad time.Time
}

type Config struct {
	Funcs         template.FuncMap
	Delimiters    []string // [left, right]
	Development   bool
	AssetVersion  string
	DefaultLayout string
	EnableCache   bool
	TemplateDir   string
	AssetDir      string
	I18n          *I18nConfig
}

type I18nConfig struct {
	DefaultLang  string
	Translations map[string]map[string]string

	currentLang string
	mu          sync.RWMutex
}

// RenderData for layout rendering
type RenderData struct {
	Layout string
	View   string
	Data   any
}

// Sparkle creates a new template with optional configuration
func Sparkle(pattern string, opts ...Option) mofu.TemplateConfig {
	cfg := &Config{
		Delimiters:  []string{"{{", "}}"},
		Funcs:       template.FuncMap{},
		TemplateDir: "templates",
		AssetDir:    "assets",
		EnableCache: true,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return &html{
		pattern: pattern,
		config:  cfg,
	}
}

type html struct {
	pattern string
	config  *Config
}

func (h *html) CreateEngine() (mofu.TemplateEngine, error) {
	t, err := h.createTemplate()
	if err != nil {
		return nil, err
	}

	engine := &HTMLTemplate{
		t:       t,
		config:  h.config,
		pattern: h.pattern,
	}

	if err := engine.Validate(); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	return engine, nil
}

func (h *html) createTemplate() (*template.Template, error) {
	t := template.New("")

	// Apply delimiters
	t = t.Delims(h.config.Delimiters[0], h.config.Delimiters[1])

	// Merge default funcs with custom funcs
	funcs := h.mergeFuncs()
	t = t.Funcs(funcs)

	// Parse templates
	pattern := filepath.Join(h.config.TemplateDir, h.pattern)
	t, err := t.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (h *html) mergeFuncs() template.FuncMap {
	funcs := defaultFuncs()

	// Add i18n functions if configured
	if h.config.I18n != nil {
		funcs["t"] = h.config.I18n.Translate
		funcs["setLang"] = h.config.I18n.SetLanguage
		funcs["currentLang"] = h.config.I18n.CurrentLanguage
	}

	// Add asset function
	if h.config.AssetDir != "" {
		funcs["asset"] = func(name string) string {
			return h.config.assetPath(name)
		}
	}

	// Merge with user-provided funcs
	maps.Copy(funcs, h.config.Funcs)
	return funcs
}

func (c *Config) assetPath(name string) string {
	if c.Development {
		return filepath.Join(c.AssetDir, name) + "?v=" + time.Now().Format("20060102150405")
	}
	if c.AssetVersion != "" {
		return filepath.Join(c.AssetDir, name) + "?v=" + c.AssetVersion
	}
	return filepath.Join(c.AssetDir, name)
}

// Default template functions
func defaultFuncs() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
		"safeURL":  func(s string) template.URL { return template.URL(s) },
		"safeJS":   func(s string) template.JS { return template.JS(s) },
		"dict": func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"partial": func(name string, data any) (template.HTML, error) {
			// This would be implemented in the Render method
			return template.HTML(""), nil
		},
	}
}

func (h *HTMLTemplate) Render(w io.Writer, name string, data any) error {
	start := time.Now()

	// Development mode: reload templates
	if h.config.Development {
		if err := h.reloadIfNeeded(); err != nil {
			return fmt.Errorf("failed to reload templates: %w", err)
		}
	}

	// Validate template existence
	if err := h.validateTemplate(name); err != nil {
		return err
	}

	// Execute the template
	err := h.t.ExecuteTemplate(w, name, data)

	// Log rendering time in development
	if h.config.Development {
		log.Printf("Template %s rendered in %v", name, time.Since(start))
	}

	return err
}

// RenderWithLayout for layout-based rendering
func (h *HTMLTemplate) RenderWithLayout(w io.Writer, renderData *RenderData) error {
	if renderData.Layout == "" && h.config.DefaultLayout != "" {
		renderData.Layout = h.config.DefaultLayout
	}

	if renderData.Layout == "" {
		return h.Render(w, renderData.View, renderData.Data)
	}

	// Create a clone to avoid modifying the original template
	tpl, err := h.t.Clone()
	if err != nil {
		return err
	}

	// Define the content block
	_, err = tpl.New("content").Parse(`{{define "content"}}{{template "` + renderData.View + `" .}}{{end}}`)
	if err != nil {
		return err
	}

	return tpl.ExecuteTemplate(w, renderData.Layout, renderData.Data)
}

func (h *HTMLTemplate) Validate() error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.t == nil {
		return errors.New("no templates loaded")
	}

	// Check if templates can be cloned (validity check)
	if _, err := h.t.Clone(); err != nil {
		return fmt.Errorf("template clone failed: %w", err)
	}

	// Check individual templates
	for _, tpl := range h.t.Templates() {
		if tpl.Tree != nil && tpl.Tree.Root == nil {
			return fmt.Errorf("template %s has nil root", tpl.Name())
		}
	}

	return nil
}

func (h *HTMLTemplate) TemplateNames() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var names []string
	for _, t := range h.t.Templates() {
		names = append(names, t.Name())
	}
	return names
}

func (h *HTMLTemplate) HasTemplate(name string) bool {
	return h.validateTemplate(name) == nil
}

func (h *HTMLTemplate) validateTemplate(name string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.t == nil {
		return errors.New("template engine not initialized")
	}

	if h.t.Lookup(name) == nil {
		return fmt.Errorf("template %s not found", name)
	}

	return nil
}

// mergeFuncs merges all template functions
func (h *HTMLTemplate) mergeFuncs() template.FuncMap {
	funcs := defaultFuncs()

	// Add i18n functions if configured
	if h.config.I18n != nil {
		funcs["t"] = h.config.I18n.Translate
		funcs["setLang"] = h.config.I18n.SetLanguage
		funcs["currentLang"] = h.config.I18n.CurrentLanguage
	}

	// Add asset function
	if h.config.AssetDir != "" {
		funcs["asset"] = func(name string) string {
			return h.config.assetPath(name)
		}
	}

	// Merge with user-provided funcs
	maps.Copy(funcs, h.config.Funcs)
	return funcs
}

// reloadIfNeeded reloads templates in development mode
func (h *HTMLTemplate) reloadIfNeeded() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	t := template.New("")

	// Apply delimiters
	t = t.Delims(h.config.Delimiters[0], h.config.Delimiters[1])

	// Apply all functions (including custom ones)
	funcs := h.mergeFuncs()
	t = t.Funcs(funcs)

	// Parse templates with correct pattern
	pattern := filepath.Join(h.config.TemplateDir, h.pattern)
	t, err := t.ParseGlob(pattern)
	if err != nil {
		return err
	}

	h.t = t
	h.lastLoad = time.Now()
	return nil
}

// I18n methods
func (i *I18nConfig) Translate(key string, args ...any) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	lang := i.currentLang
	if lang == "" {
		lang = i.DefaultLang
	}

	translations, exists := i.Translations[lang]
	if !exists {
		return key
	}

	translation, exists := translations[key]
	if !exists {
		return key
	}

	if len(args) > 0 {
		return fmt.Sprintf(translation, args...)
	}

	return translation
}

func (i *I18nConfig) SetLanguage(lang string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.currentLang = lang
}

func (i *I18nConfig) CurrentLanguage() string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.currentLang
}

func SafeHTML(s string) template.HTML {
	return template.HTML(s)
}

func SafeURL(s string) template.URL {
	return template.URL(s)
}

func SafeJS(s string) template.JS {
	return template.JS(s)
}
