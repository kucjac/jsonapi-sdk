package ginjsonapi

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kucjac/jsonapi-sdk"
)

func RouteHandler(router *gin.Engine, handler *jsonapisdk.JSONAPIHandler) error {
	for _, model := range handler.ModelHandlers {
		// mStruct := handler.Controller.Models.Get(model.ModelType)
		mStruct := handler.Controller.Models.Get(model.ModelType)

		if mStruct == nil {
			return fmt.Errorf("Model:'%s' not precomputed.", model.ModelType.Name())
		}

		base := handler.Controller.APIURLBase + "/" + mStruct.GetCollectionType()

		// CREATE
		if model.Create != nil {
			router.POST(base, gin.WrapF(handler.Create(model)))
		} else {
			router.POST(base, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Create)))
		}

		// GET
		if model.Get != nil {
			router.GET(base+"/:id", gin.WrapF(handler.Get(model)))
		} else {
			router.GET(base+"/:id", gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Get)))

		}

		// LIST
		if model.List != nil {
			router.GET(base, gin.WrapF(handler.List(model)))
		} else {
			router.GET(base, gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.List)))
		}

		// PATCH
		if model.Patch != nil {
			router.PATCH(base+"/:id", gin.WrapF(handler.Patch(model)))
		} else {
			router.PATCH(base+"/:id", gin.WrapF(handler.EndpointForbidden(model, jsonapisdk.Patch)))
		}

		// DELETE
		if model.Delete != nil {
			router.DELETE(base+"/:id", gin.WrapF(handler.Delete(model)))
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
				router.GET(base+"/:id/"+rel, gin.WrapF(handler.GetRelated(model)))
				router.GET(base+"/:id/relationships/"+rel, gin.WrapF(handler.GetRelationship(model)))
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
