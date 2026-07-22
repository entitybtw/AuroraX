//go:build swagger

package main

import swaggerdocs "aurora/apps/aurora/docs"

func configureSwaggerDocs(basePath string) {
	swaggerdocs.SwaggerInfo.BasePath = basePath
}
