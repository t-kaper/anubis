package common

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func UriJsonHandler[CTX *gin.Context, U any, B any, R any](
	ctx *gin.Context,
	processFunc func(CTX, *U, *B) (R, error),
) {
	var uri U
	var body B

	if err := ctx.ShouldBindUri(&uri); err != nil {
		ValidateErrorResponse(ctx, uri, err)
		return
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ValidateErrorResponse(ctx, body, err)
		return
	}

	// Обработка логики
	res, err := processFunc(ctx, &uri, &body)
	if err != nil {
		HandlerError(ctx, err)
		return
	}

	// Успешный ответ

	APIResponse(ctx, http.StatusOK, res)
}

func UriQueryHandler[CTX *gin.Context, U any, B any, R any](
	ctx *gin.Context,
	processFunc func(CTX, *U, *B) (R, error),
) {
	var uri U
	var body B

	if err := ctx.ShouldBindUri(&uri); err != nil {
		ValidateErrorResponse(ctx, uri, err)
		return
	}
	if err := ctx.ShouldBindQuery(&body); err != nil {
		ValidateErrorResponse(ctx, body, err)
		return
	}

	// Обработка логики
	res, err := processFunc(ctx, &uri, &body)
	if err != nil {
		HandlerError(ctx, err)
		return
	}

	// Успешный ответ

	APIResponse(ctx, http.StatusOK, res)
}

func UriHandler[CTX *gin.Context, U any, R any](
	ctx *gin.Context,
	processFunc func(CTX, *U) (R, error),
) {
	var uri U

	if err := ctx.ShouldBindUri(&uri); err != nil {
		ValidateErrorResponse(ctx, uri, err)
		return
	}

	// Обработка логики
	res, err := processFunc(ctx, &uri)
	if err != nil {
		HandlerError(ctx, err)
		return
	}
	// Успешный ответ
	APIResponse(ctx, http.StatusOK, res)
}

func JsonHandler[CTX *gin.Context, B any, R any](
	ctx *gin.Context,
	processFunc func(CTX, *B) (R, error),
) {
	var body B

	if err := ctx.ShouldBindJSON(&body); err != nil {
		ValidateErrorResponse(ctx, body, err)
		return
	}

	// Обработка логики
	res, err := processFunc(ctx, &body)
	if err != nil {
		HandlerError(ctx, err)
		return
	}

	// Успешный ответ
	APIResponse(ctx, http.StatusOK, res)
}
