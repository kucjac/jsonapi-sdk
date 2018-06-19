package ginjsonapi

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kucjac/jsonapi-sdk"
	"net/http"
)

func RouteHandler(router *gin.Engine, handler *jsonapisdk.JSONAPIHandler) error {
	for _, model := range handler.ModelHandlers {
		// mStruct := handler.Controller.Models.Get(model.ModelType)
		mStruct := handler.Controller.Models.Get(model.ModelType)

		if mStruct == nil {
			return fmt.Errorf("Model:'%s' not precomputed.", model.ModelType.Name())
		}

		base := handler.Controller.APIURLBase + "/" + mStruct.GetCollectionType()

		getMiddlewares := func(middlewares ...jsonapisdk.MiddlewareFunc) gin.HandlersChain {
			ginMiddlewares := []gin.HandlerFunc{}
			for _, middleware := range middlewares {
				ginMiddlewares = append(ginMiddlewares, wrap(middleware))
			}
			return ginMiddlewares
		}

		var handlers gin.HandlersChain
		var handlerFunc gin.HandlerFunc
		// CREATE
		if model.Create != nil {
			handlerFunc = gin.WrapF(handler.Create(model))
			handlers = getMiddlewares(model.Create.Middlewares...)
			handlers = append(handlers, handlerFunc)

			router.POST(base, handlers...)

		} else {
			router.POST(base, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Create)))
		}

		// GET
		if model.Get != nil {
			handlerFunc = gin.WrapF(handler.Get(model))
			handlers = getMiddlewares(model.Get.Middlewares...)
			handlers = append(handlers, handlerFunc)
			router.GET(base+"/:id", handlers...)
		} else {
			router.GET(base+"/:id", gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Get)))

		}

		// LIST
		if model.List != nil {
			handlerFunc = gin.WrapF(handler.List(model))
			handlers = getMiddlewares(model.List.Middlewares...)
			handlers = append(handlers, handlerFunc)
			router.GET(base, handlers...)
		} else {
			router.GET(base, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.List)))
		}

		// PATCH
		if model.Patch != nil {
			handlerFunc = gin.WrapF(handler.Patch(model))
			handlers = getMiddlewares(model.Patch.Middlewares...)
			handlers = append(handlers, handlerFunc)
			router.PATCH(base+"/:id", handlers...)
		} else {
			router.PATCH(base+"/:id", gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Patch)))
		}

		// DELETE
		if model.Delete != nil {
			handlerFunc = gin.WrapF(handler.Delete(model))
			handlers = getMiddlewares(model.Delete.Middlewares...)
			handlers = append(handlers, handlerFunc)
			router.DELETE(base+"/:id", handlers...)
		} else {
			router.DELETE(base+"/:id", gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Delete)))
		}

		// RELATIONSHIP & RELATED
		for _, rel := range mStruct.ListRelationshipNames() {
			// POST
			router.POST(base+"/:id/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Create)))
			router.POST(base+"/:id/relationships/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Create)))

			// GETS
			if model.Get != nil {
				handlerFunc = gin.WrapF(handler.GetRelated(model))
				handlers = getMiddlewares(model.Get.Middlewares...)
				handlers = append(handlers, handlerFunc)
				router.GET(base+"/:id/"+rel, handlers...)

				handlerFunc = gin.WrapF(handler.GetRelationship(model))
				handlers = getMiddlewares(model.Get.Middlewares...)
				handlers = append(handlers, handlerFunc)
				router.GET(base+"/:id/relationships/"+rel, handlers...)
			} else {
				router.GET(base+"/:id/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Get)))
				router.GET(base+"/:id/relationships/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Get)))
			}

			// PATCH
			router.PATCH(base+"/:id/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Patch)))
			router.PATCH(base+"/:id/relationships/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Patch)))

			router.DELETE(base+"/:id/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Delete)))
			router.DELETE(base+"/:id/relationships/"+rel, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Delete)))

		}

	}
	return nil
}

func wrap(f func(h http.Handler) http.Handler) gin.HandlerFunc {
	next, adapter := new()
	return adapter(f(next))
}

func new() (http.Handler, func(h http.Handler) gin.HandlerFunc) {
	nextHandler := new(connectHandler)
	makeGinHandler := func(h http.Handler) gin.HandlerFunc {
		return func(c *gin.Context) {
			state := &middlewareCtx{ctx: c}
			ctx := context.WithValue(c.Request.Context(), nextHandler, state)
			h.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
			if !state.childCalled {
				c.Abort()
			}
		}
	}
	return nextHandler, makeGinHandler
}
