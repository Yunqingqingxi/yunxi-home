package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

//go:embed swagger-ui/*
var swaggerFS embed.FS

// swaggerUIFS strips the "swagger-ui" prefix so paths look like /swagger-ui-bundle.js.
var swaggerUIFS, _ = fs.Sub(swaggerFS, "swagger-ui")

// loadOpenAPIYAML reads the embedded YAML spec at startup.
func loadOpenAPIYAML() []byte {
	data, err := fs.ReadFile(swaggerUIFS, "openapi.yaml")
	if err != nil {
		// Fallback: return empty spec — UI will show error
		return []byte("openapi: \"3.0.3\"\ninfo:\n  title: Error\n  version: \"0\"\npaths: {}\n")
	}
	return data
}

// swaggerHTML is the HTML page for Swagger UI, served from embedded assets.
const swaggerHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Yunxi Home API — Swagger</title>
  <link rel="icon" type="image/png" href="/api/swagger/favicon-32x32.png">
  <link rel="stylesheet" href="/api/swagger/swagger-ui.css">
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { background: #1a1a2e; }
    .topbar {
      background: #16213e; border-bottom: 1px solid #0f3460;
      padding: 12px 24px; display: flex; align-items: center; gap: 12px;
    }
    .topbar h1 { font-size: 18px; font-weight: 600; color: #e94560; }
    .topbar span { font-size: 12px; color: #888; }
    .topbar a { text-decoration: none; font-size: 13px; padding: 4px 12px; border-radius: 4px; background: #e94560; color: #fff; margin-left: auto; }
    .topbar-links { display: flex; gap: 8px; margin-left: auto; }
    .topbar-links a { text-decoration: none; font-size: 13px; padding: 4px 12px; border-radius: 4px; }
    .btn-yaml { background: #e94560; color: #fff; }
    .btn-json { background: #0f3460; color: #ccc; }
    .container { max-width: 1400px; margin: 0 auto; }
    #swagger-ui { padding: 0 16px 40px; }
    .swagger-ui { filter: invert(88%) hue-rotate(180deg); }
    .swagger-ui .microlight { filter: invert(100%) hue-rotate(180deg); }
  </style>
</head>
<body>
  <div class="topbar">
    <h1>&#x1F3E0; Yunxi Home</h1>
    <span>API v3.2.0</span>
    <div class="topbar-links">
      <a class="btn-json" href="/api/swagger/openapi.yaml">YAML</a>
    </div>
  </div>
  <div class="container">
    <div id="swagger-ui"></div>
  </div>
  <script src="/api/swagger/swagger-ui-bundle.js" crossorigin="anonymous"></script>
  <script src="/api/swagger/swagger-ui-standalone-preset.js" crossorigin="anonymous"></script>
  <script>
    SwaggerUIBundle({
      url: '/api/swagger/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      docExpansion: 'list',
      filter: true,
      showExtensions: true,
      tryItOutEnabled: true,
      persistAuthorization: true,
      withCredentials: true,
      defaultModelsExpandDepth: 1,
      defaultModelExpandDepth: 1,
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
      plugins: [SwaggerUIBundle.plugins.DownloadUrl],
      layout: 'StandaloneLayout',
    });
  </script>
</body>
</html>`

// RegisterSwaggerRoutes adds fully-offline Swagger UI and OpenAPI spec endpoints.
// All assets are embedded in the binary — no CDN or internet required.
func RegisterSwaggerRoutes(api *echo.Group) {
	// Swagger UI page
	api.GET("/swagger", func(c echo.Context) error {
		return c.HTML(http.StatusOK, swaggerHTML)
	})

	// OpenAPI YAML (data source for Swagger UI)
	api.GET("/swagger/openapi.yaml", func(c echo.Context) error {
		return c.Blob(http.StatusOK, "application/yaml; charset=utf-8", loadOpenAPIYAML())
	})

	// Serve embedded Swagger UI static files (JS, CSS, PNG)
	api.GET("/swagger/*", func(c echo.Context) error {
		p := strings.TrimPrefix(c.Request().URL.Path, "/api/swagger/")
		if p == "" {
			return c.Redirect(http.StatusMovedPermanently, "/api/swagger")
		}
		// map .js → correct MIME
		ext := filepath.Ext(p)
		ct := "application/octet-stream"
		switch ext {
		case ".js":
			ct = "application/javascript; charset=utf-8"
		case ".css":
			ct = "text/css; charset=utf-8"
		case ".png":
			ct = "image/png"
		}
		data, err := fs.ReadFile(swaggerUIFS, p)
		if err != nil {
			return c.String(http.StatusNotFound, "not found")
		}
		return c.Blob(http.StatusOK, ct, data)
	})
}
