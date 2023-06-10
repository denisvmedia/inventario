// Code generated by swaggo/swag. DO NOT EDIT.

package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {
            "name": "API Support",
            "url": "https://github.com/denisvmedia/inventario/issues",
            "email": "ask@artprima.cz"
        },
        "license": {
            "name": "MIT"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/areas": {
            "get": {
                "description": "get areas",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "areas"
                ],
                "summary": "List areas",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.AreasResponse"
                        }
                    }
                }
            },
            "post": {
                "description": "add by area data",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "areas"
                ],
                "summary": "Create a new area",
                "parameters": [
                    {
                        "description": "Area object",
                        "name": "area",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/jsonapi.AreaRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Area created",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.AreaResponse"
                        }
                    },
                    "404": {
                        "description": "Area not found",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    },
                    "422": {
                        "description": "User-side request problem",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    }
                }
            }
        },
        "/areas/{id}": {
            "get": {
                "description": "get area by ID",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "areas"
                ],
                "summary": "Get a area",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Area ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.AreaResponse"
                        }
                    }
                }
            },
            "put": {
                "description": "Update by area data",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "areas"
                ],
                "summary": "Update a area",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Area ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "Area object",
                        "name": "area",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/jsonapi.AreaRequest"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.AreaResponse"
                        }
                    },
                    "404": {
                        "description": "Area not found",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    },
                    "422": {
                        "description": "User-side request problem",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    }
                }
            },
            "delete": {
                "description": "Delete by area ID",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "areas"
                ],
                "summary": "Delete a area",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Area ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No content"
                    },
                    "404": {
                        "description": "Area not found",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    }
                }
            }
        },
        "/locations": {
            "get": {
                "description": "get locations",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "locations"
                ],
                "summary": "List locations",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.LocationsResponse"
                        }
                    }
                }
            },
            "post": {
                "description": "add by location data",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "locations"
                ],
                "summary": "Create a new location",
                "parameters": [
                    {
                        "description": "Location object",
                        "name": "location",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/jsonapi.LocationRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Location created",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.LocationResponse"
                        }
                    },
                    "404": {
                        "description": "Location not found",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    },
                    "422": {
                        "description": "User-side request problem",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    }
                }
            }
        },
        "/locations/{id}": {
            "get": {
                "description": "get location by ID",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "locations"
                ],
                "summary": "Get a location",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Location ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.LocationResponse"
                        }
                    }
                }
            },
            "put": {
                "description": "Update by location data",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "locations"
                ],
                "summary": "Update a location",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Location ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "Location object",
                        "name": "location",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/jsonapi.LocationRequest"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.LocationResponse"
                        }
                    },
                    "404": {
                        "description": "Location not found",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    },
                    "422": {
                        "description": "User-side request problem",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    }
                }
            },
            "delete": {
                "description": "Delete by location ID",
                "consumes": [
                    "application/vnd.api+json"
                ],
                "produces": [
                    "application/vnd.api+json"
                ],
                "tags": [
                    "locations"
                ],
                "summary": "Delete a location",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Location ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No content"
                    },
                    "404": {
                        "description": "Location not found",
                        "schema": {
                            "$ref": "#/definitions/jsonapi.Errors"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "jsonapi.AreaRequest": {
            "type": "object",
            "properties": {
                "data": {
                    "$ref": "#/definitions/models.Area"
                }
            }
        },
        "jsonapi.AreaResponse": {
            "type": "object",
            "properties": {
                "attributes": {
                    "$ref": "#/definitions/models.Area"
                },
                "id": {
                    "type": "string"
                },
                "type": {
                    "type": "string",
                    "enum": [
                        "areas"
                    ],
                    "example": "areas"
                }
            }
        },
        "jsonapi.AreasMeta": {
            "type": "object",
            "properties": {
                "areas": {
                    "type": "integer",
                    "format": "int64",
                    "example": 1
                }
            }
        },
        "jsonapi.AreasResponse": {
            "type": "object",
            "properties": {
                "data": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/models.Area"
                    }
                },
                "meta": {
                    "$ref": "#/definitions/jsonapi.AreasMeta"
                }
            }
        },
        "jsonapi.Error": {
            "type": "object",
            "properties": {
                "status": {
                    "description": "user-level status message",
                    "type": "string"
                }
            }
        },
        "jsonapi.Errors": {
            "type": "object",
            "properties": {
                "errors": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/jsonapi.Error"
                    }
                }
            }
        },
        "jsonapi.LocationRequest": {
            "type": "object",
            "properties": {
                "data": {
                    "$ref": "#/definitions/models.Location"
                }
            }
        },
        "jsonapi.LocationResponse": {
            "type": "object",
            "properties": {
                "attributes": {
                    "$ref": "#/definitions/models.Location"
                },
                "id": {
                    "type": "string"
                },
                "type": {
                    "type": "string",
                    "enum": [
                        "locations"
                    ],
                    "example": "locations"
                }
            }
        },
        "jsonapi.LocationsMeta": {
            "type": "object",
            "properties": {
                "locations": {
                    "type": "integer",
                    "format": "int64",
                    "example": 1
                }
            }
        },
        "jsonapi.LocationsResponse": {
            "type": "object",
            "properties": {
                "data": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/models.Location"
                    }
                },
                "meta": {
                    "$ref": "#/definitions/jsonapi.LocationsMeta"
                }
            }
        },
        "models.Area": {
            "type": "object",
            "properties": {
                "id": {
                    "type": "string"
                },
                "location_id": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                }
            }
        },
        "models.Location": {
            "type": "object",
            "properties": {
                "address": {
                    "type": "string"
                },
                "id": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	Host:             "",
	BasePath:         "/api/v1",
	Schemes:          []string{},
	Title:            "Inventario API",
	Description:      "This is an Inventario daemon.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}