package common

import (
	schemes2 "anubis/app/core/schemes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/mongo"
)

type Empty struct {
}

func APIResponse(ctx *gin.Context, StatusCode int, data interface{}) {
	jsonResponse := schemes2.Responses{
		StatusCode: StatusCode,
		Data:       data,
	}
	ctx.JSON(StatusCode, jsonResponse)
}

func getJSONTag(body interface{}, fieldName string) string {
	t := reflect.TypeOf(body)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	field, _ := t.FieldByName(fieldName)
	jsonTag := field.Tag.Get("json")
	formTag := field.Tag.Get("form")
	if jsonTag != "" {
		return strings.Split(jsonTag, ",")[0]
	}
	if formTag != "" {
		return strings.Split(formTag, ",")[0]
	}
	return fieldName
}

// ErrorResponse — единая точка маппинга внутренних кодов (schemes.HTTPError.Code)
// в HTTP статус. Важно:
// - 4xx: ожидаемые ошибки (клиент может поправить запрос/токен/права)
// - 5xx: внутренние ошибки (БД/конфиг/баг) — клиент НЕ может починить сам
func ErrorResponse(ctx *gin.Context, code int, error string) {
	err := schemes2.HTTPError{
		Code: code,
		Err:  error,
	}
	status := httpStatusFromLegacyCode(code)
	ctx.AbortWithStatusJSON(status, err)
}

func httpStatusFromLegacyCode(code int) int {
	// Fallback для “старых” ошибок, где ещё не выставляется schemes.ErrorResponse.Kind.
	switch code {
	case 102, 105, 106, 107:
		return http.StatusInternalServerError
	case 97:
		return http.StatusUnauthorized
	}

	// По умолчанию — 400 (валидация/плохие параметры).
	return http.StatusBadRequest
}

func HandlerError(ctx *gin.Context, err error) {

	var _ = ctx.Error(err)
	var mongoErr mongo.WriteException
	var errResp *schemes2.ErrorResponse

	if errors.As(err, &mongoErr) {
		for _, writeErr := range mongoErr.WriteErrors {
			switch writeErr.Code {
			case 11000: // Duplicate key error code
				ErrorResponse(ctx, 103, "Document already exists")
			default:
				ErrorResponse(ctx, 102, "Database error")
			}
		}
	} else if errors.As(err, &errResp) {
		// Точный маппинг HTTP статуса: берём из enum Kind (если выставлен конструкторами schemes.*).
		status := httpStatusFromLegacyCode(errResp.Code)
		if errResp.Kind != 0 {
			status = errResp.Kind.HTTPStatus()
		}
		ctx.AbortWithStatusJSON(status, schemes2.HTTPError{Code: errResp.Code, Err: errResp.Err})

	} else {
		ErrorResponse(ctx, 99, "Very strange error. please write to the administrator.")
	}
}

//func handleMongoError(ctx *gin.Context, mongoErr mongo.WriteException) {
//	for _, writeErr := range mongoErr.WriteErrors {
//		if writeErr.Code == 11000 {
//			ErrorResponse(ctx, 103, "Duplicate entry")
//			return
//		}
//	}
//	ErrorResponse(ctx, 102, "Database operation failed")
//}

func handleJSONErrors(ctx *gin.Context, jsonErr *json.UnmarshalTypeError) {
	ctx.AbortWithStatusJSON(http.StatusBadRequest, schemes2.ValidateErrorResponse{
		Code: 100,
		Err: []schemes2.ValidateError{{
			Field: jsonErr.Field,
			Msg:   fmt.Sprintf("Expected type %s for field %s", jsonErr.Type, jsonErr.Field),
		}},
	})
}

func handleValidationErrors(ctx *gin.Context, body interface{}, ve validator.ValidationErrors) {
	out := make([]schemes2.ValidateError, len(ve))
	for i, fe := range ve {

		jsonField := getJSONTag(body, fe.StructField())
		out[i] = schemes2.ValidateError{
			Field: jsonField,
			Msg:   msgForTag(fe),
		}
	}
	ctx.AbortWithStatusJSON(http.StatusBadRequest, schemes2.ValidateErrorResponse{
		Code: 100,
		Err:  out,
	})
}

func ValidateErrorResponse(ctx *gin.Context, body interface{}, err error) {
	var _ = ctx.Error(err)
	var ve validator.ValidationErrors
	var jsonErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &ve):
		handleValidationErrors(ctx, body, ve)
	case errors.As(err, &jsonErr):
		handleJSONErrors(ctx, jsonErr)
	default:
		//TODO: узнать что за ошибка
		ErrorResponse(ctx, 101, "Invalid request format")
	}
}
