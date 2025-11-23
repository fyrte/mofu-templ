package html

import (
	"html/template"
	"maps"
)

// Option is a functional option for configuring the template engine
type Option func(*Config)

// WithDevelopment enables development mode with auto-reload
func WithDevelopment(dev bool) Option {
	return func(c *Config) {
		c.Development = dev
	}
}

// WithTemplateDir sets the template directory
func WithTemplateDir(dir string) Option {
	return func(c *Config) {
		c.TemplateDir = dir
	}
}

// WithAssetDir sets the asset directory
func WithAssetDir(dir string) Option {
	return func(c *Config) {
		c.AssetDir = dir
	}
}

// WithFuncs adds custom template functions
func WithFuncs(funcs template.FuncMap) Option {
	return func(c *Config) {
		if c.Funcs == nil {
			c.Funcs = template.FuncMap{}
		}
		maps.Copy(c.Funcs, funcs)
	}
}

// WithDelimiters sets custom template delimiters
func WithDelimiters(left, right string) Option {
	return func(c *Config) {
		c.Delimiters = []string{left, right}
	}
}

// WithAssetVersion sets the asset version for cache busting
func WithAssetVersion(version string) Option {
	return func(c *Config) {
		c.AssetVersion = version
	}
}

// WithDefaultLayout sets the default layout template
func WithDefaultLayout(layout string) Option {
	return func(c *Config) {
		c.DefaultLayout = layout
	}
}

// WithCache enables or disables template caching
func WithCache(enable bool) Option {
	return func(c *Config) {
		c.EnableCache = enable
	}
}

// WithI18n configures internationalization
func WithI18n(defaultLang string, translations map[string]map[string]string) Option {
	return func(c *Config) {
		c.I18n = &I18nConfig{
			DefaultLang:  defaultLang,
			Translations: translations,
			currentLang:  defaultLang,
		}
	}
}
