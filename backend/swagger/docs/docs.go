// Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/api/search": {
            "get": {
                "description": "Searches for files matching the provided query. Returns file paths and metadata based on the user's session and scope.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "search"
                ],
                "summary": "Search Files",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Search query",
                        "name": "query",
                        "in": "query",
                        "required": true
                    },
                    {
                        "type": "string",
                        "description": "path within user scope to search, for example '/first/second' to search within the second directory only",
                        "name": "scope",
                        "in": "query"
                    },
                    {
                        "type": "string",
                        "description": "User session ID, add unique value to prevent collisions",
                        "name": "SessionId",
                        "in": "header"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "List of search results",
                        "schema": {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/files.searchResult"
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "additionalProperties": {
                                "type": "string"
                            }
                        }
                    }
                }
            }
        },
        "/health": {
            "get": {
                "description": "Returns the health status of the API.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "health"
                ],
                "summary": "Health Check",
                "responses": {
                    "200": {
                        "description": "successful health check response",
                        "schema": {
                            "$ref": "#/definitions/http.HealthCheckResponse"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "files.searchResult": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string"
                },
                "size": {
                    "type": "integer"
                },
                "type": {
                    "type": "string"
                }
            }
        },
        "http.HealthCheckResponse": {
            "description": "Response structure for health check",
            "type": "object",
            "properties": {
                "status": {
                    "description": "The status of the health check",
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "",
	Description:      "",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
