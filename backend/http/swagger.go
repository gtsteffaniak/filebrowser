package http

import (
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"
)

func swaggerHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Api {
		return http.StatusForbidden, nil
	}
	httpSwagger.Handler(
		httpSwagger.URL(config.Server.BaseURL+"swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	).ServeHTTP(w, r)
	return http.StatusOK, nil
}
