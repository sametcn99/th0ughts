package server

import "net/http"

type spec = map[string]any

func (s *Server) openapi(w http.ResponseWriter, r *http.Request) {
	paths := spec{}
	auth := []spec{{"bearer": []string{}}, {"session": []string{}}}
	schemaFor := func(name string) any {
		for _, c := range commands {
			if c.Name == name {
				return c.Schema
			}
		}
		return spec{"type": "object"}
	}
	operation := func(name, summary string, schema any, private bool) spec {
		v := spec{"operationId": name, "summary": summary, "responses": spec{"200": spec{"description": "Operation completed"}, "400": spec{"description": "Invalid input"}, "403": spec{"description": "Insufficient access"}, "404": spec{"description": "Record not found"}, "409": spec{"description": "Record revision conflict"}}}
		if schema != nil {
			v["requestBody"] = spec{"required": true, "content": spec{"application/json": spec{"schema": schema}}}
		}
		if private {
			v["security"] = auth
		}
		return v
	}
	for _, op := range commands {
		paths["/api/v1/"+op.Name] = spec{"post": operation(op.Name, s.catalog["en"][op.Name], op.Schema, op.Scope != "")}
	}
	list := operation("listPosts", "List the timeline", nil, false)
	var params []spec
	for _, key := range []string{"q", "lang", "tag", "cursor", "limit", "private"} {
		kind := "string"
		if key == "limit" {
			kind = "integer"
		}
		if key == "private" {
			kind = "boolean"
		}
		params = append(params, spec{"in": "query", "name": key, "schema": spec{"type": kind}})
	}
	list["parameters"] = params
	paths["/api/v1/posts"] = spec{"get": list, "post": operation("createPost", "Create a post", schemaFor("save_post"), true)}
	paths["/api/v1/posts/{id}"] = spec{"parameters": []spec{{"in": "path", "name": "id", "required": true, "schema": spec{"type": "string"}}}, "get": operation("getPost", "Read a post", nil, false), "put": operation("updatePost", "Update a post with its current revision", schemaFor("save_post"), true), "delete": operation("deletePost", "Delete a post", nil, true)}
	for _, name := range []string{"upload", "restore", "backup"} {
		verb := "post"
		mime := "application/octet-stream"
		op := operation(name, name, nil, true)
		if name == "backup" {
			verb = "get"
			op["responses"] = spec{"200": spec{"description": "Portable ZIP archive", "content": spec{"application/zip": spec{"schema": spec{"type": "string", "format": "binary"}}}}}
		} else {
			if name == "restore" {
				mime = "application/zip"
			}
			op["requestBody"] = spec{"required": true, "content": spec{mime: spec{"schema": spec{"type": "string", "format": "binary"}}}}
		}
		paths["/api/v1/"+name] = spec{verb: op}
	}
	paths["/api/v1/tools"] = spec{"get": operation("listTools", "Discover tools available to the current principal", nil, false)}
	login := operation("signIn", "Create an owner session", spec{"type": "object", "required": []string{"password"}, "properties": spec{"password": spec{"type": "string", "format": "password"}, "code": spec{"type": "string"}}}, false)
	paths["/auth/login"] = spec{"post": login}
	respond(w, spec{"openapi": "3.1.0", "info": spec{"title": "th0ughts", "version": "1.0.0", "description": "For cookie-authenticated writes send X-Th0ughts-Request: 1. Bearer tokens grant individual scopes."}, "components": spec{"securitySchemes": spec{"bearer": spec{"type": "http", "scheme": "bearer"}, "session": spec{"type": "apiKey", "in": "cookie", "name": "session"}}}, "paths": paths})
}
