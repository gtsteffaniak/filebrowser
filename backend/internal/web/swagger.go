package web

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"

)

func swaggerHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.User.Permissions.Api {
		return http.StatusForbidden, nil
	}
	httpSwagger.Handler(
		httpSwagger.URL(settings.Config.Http.BaseURL+"swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	).ServeHTTP(w, r)
	return http.StatusOK, nil
}
