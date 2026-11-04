package openapi

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newRegister returns the [flam.IRestEndpointRegister] of the OpenAPI
// endpoints, of the group, the paths, the ids, the info, the servers and
// the Swagger UI settings of the configuration, or of their defaults. It
// is the provider registered by the bundle returned from [NewBundle].
func newRegister(
	config flam.IConfig,
) flam.IRestEndpointRegister {
	var servers []Server
	for _, url := range config.StringSlice(ServersConfigPath) {
		servers = append(servers, Server{URL: url})
	}
	return &register{
		group: config.String(GroupConfigPath, defaultGroup),
		path:  config.String(PathConfigPath, defaultPath),
		id:    config.Int(IdConfigPath, defaultId),
		info: Info{
			Title:       config.String(TitleConfigPath, defaultTitle),
			Version:     config.String(VersionConfigPath, defaultVersion),
			Description: config.String(DescriptionConfigPath),
		},
		servers: servers,
		ui:      config.Bool(UIConfigPath),
		uiPath:  config.String(UIPathConfigPath, defaultUIPath),
		uiId:    config.Int(UIIdConfigPath, defaultUIId),
		uiCDN:   config.String(UICDNConfigPath, defaultUICDN),
	}
}

// register is the [flam.IRestEndpointRegister] of the OpenAPI endpoints.
type register struct {
	group   string
	path    string
	id      int
	info    Info
	servers []Server
	ui      bool
	uiPath  string
	uiId    int
	uiCDN   string
}

// compile time check that *register implements IRestEndpointRegister
var _ flam.IRestEndpointRegister = (*register)(nil)

func (r *register) Group() string {
	return r.group
}

// Register registers the endpoint serving, to the GET requests of its
// path, the JSON of the OpenAPI document of the endpoints of the router,
// read at the first request, once every register registered its own, so
// that the order of the registers does not matter, then the endpoint of
// the Swagger UI page, when enabled. It fails with ErrRouter when the
// router does not list its endpoints, and with the error of the router
// when a path is not a valid one.
func (r *register) Register(
	router flam.IRestRouter,
) error {
	lister, ok := router.(flam.IRestEndpointLister)
	if !ok {
		return flam.NewWrappedError(ErrRouter, registerRegisterMessage)
	}

	var once sync.Once
	var document []byte
	if err := router.Handle(flam.RestEndpoint{
		Id:      r.id,
		Pattern: http.MethodGet + " " + r.path,
		Summary: "OpenAPI document",
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		once.Do(func() {
			// a document holds no value encoding/json cannot write
			document, _ = json.Marshal(NewDocument(r.info, r.servers, lister.Endpoints()))
		})
		w.Header().Set("Content-Type", contentTypeJSON)
		_, _ = w.Write(document)
	})); err != nil {
		return err
	}

	if !r.ui {
		return nil
	}
	var page bytes.Buffer
	// a buffer cannot fail to be written
	_ = uiPage.Execute(&page, map[string]string{"Title": r.info.Title, "CDN": r.uiCDN, "URL": r.path})
	return router.Handle(flam.RestEndpoint{
		Id:      r.uiId,
		Pattern: http.MethodGet + " " + r.uiPath,
		Summary: "Swagger UI page",
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page.Bytes())
	}))
}

// registerRegisterMessage is the message of the errors returned by
// Register.
const registerRegisterMessage = "unable to register openapi endpoints"

// uiPage is the template of the Swagger UI page, loading the files of the
// CDN, and the document of the URL.
var uiPage = template.Must(template.New("ui").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<link rel="stylesheet" href="{{.CDN}}/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="{{.CDN}}/swagger-ui-bundle.js" crossorigin></script>
<script>
window.onload = function () {
  window.ui = SwaggerUIBundle({url: {{.URL}}, dom_id: "#swagger-ui"});
};
</script>
</body>
</html>
`))
