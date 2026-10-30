package schemes

import (
	"errors"
	"net/http"
	"runtime"
	"runtime/debug"
	"strconv"
)

// ErrKind — “enum” для того, чтобы точно (без парсинга текста) маппить наши ошибки на HTTP статусы.
// Это важнее, чем пытаться вывести статус из Code/Err строк.
type ErrKind uint8

const (
	KindBadRequest ErrKind = iota + 1
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindGone
	KindInternal
)

func (k ErrKind) HTTPStatus() int {
	switch k {
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindGone:
		return http.StatusGone
	case KindInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// ValidateError --------------------------------
type ValidateError struct {
	Field string
	Msg   string
}

type ValidateErrorResponse struct {
	Code int             `json:"code"`
	Err  []ValidateError `json:"error"`
}

// ErrorResponse --------------------------------
type ErrorResponse struct {
	Code    int
	Err     string
	ErrBase error
	// Kind определяет HTTP статус (401/403/404/409/410/500) без эвристик по строкам.
	// В API-ответ не отдаём — только для маппинга статуса и для логов.
	Kind ErrKind
	// Origin — место, где ошибка была создана (файл:строка func).
	// Используется ТОЛЬКО для логов (в API-ответ не отдаём).
	Origin string
}

func (e *ErrorResponse) Error() string {
	// 1. ЗАЩИТА ОТ TYPED NIL
	if e == nil {
		// Это сигнал для тебя, что где-то в коде вернули типизированный nil.
		// Но приложение при этом не упадет. is nil (typed nil error)
		return "CRITICAL_BUG (( "
	}

	// 2. ЗАЩИТА ОТ ОТСУТСТВИЯ ErrBase
	baseMsg := "Empty"
	if e.ErrBase != nil {
		baseMsg = e.ErrBase.Error()
	}

	// 3. Формирование строки
	return "[Code_" + strconv.Itoa(e.Code) + "] " + e.Err + " | Base: " + baseMsg
}

// stackError хранит стек на момент оборачивания.
type stackError struct {
	err   error
	stack []byte
}

func (e *stackError) Error() string { return e.err.Error() }
func (e *stackError) Unwrap() error { return e.err }

func withStack(err error) error {
	if err == nil {
		return nil
	}
	// Если уже есть стек — не оборачиваем повторно.
	var se *stackError
	if errors.As(err, &se) {
		return err
	}
	return &stackError{err: err, stack: debug.Stack()}
}

// StackString извлекает стек, если ошибка была обёрнута через WrapInternal.
func StackString(err error) (string, bool) {
	var se *stackError
	if errors.As(err, &se) && se != nil && len(se.stack) > 0 {
		return string(se.stack), true
	}
	return "", false
}

func callerOrigin(skip int) string {
	// skip: 0 = callerOrigin, 1 = WrapInternal, 2 = место вызова WrapInternal.
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return file + ":" + strconv.Itoa(line)
	}
	return file + ":" + strconv.Itoa(line) + " " + fn.Name()
}

func newKind(kind ErrKind, code int, msg string, base error, stack bool, originSkip int) *ErrorResponse {
	var wrapped error
	if stack {
		wrapped = withStack(base)
	} else {
		wrapped = base
	}
	return &ErrorResponse{
		Code:    code,
		Err:     msg,
		ErrBase: wrapped,
		Kind:    kind,
		Origin:  callerOrigin(originSkip),
	}
}

// NewKind — 4xx/5xx ошибка с origin, без stack. Kind определяет HTTP статус.
func NewKind(kind ErrKind, code int, msg string) *ErrorResponse {
	// callerOrigin: 0=callerOrigin,1=newKind,2=NewKind,3=call site
	return newKind(kind, code, msg, nil, false, 3)
}

// WrapKind — 4xx ошибка с origin и base (без stack).
func WrapKind(kind ErrKind, code int, msg string, base error) *ErrorResponse {
	// callerOrigin: 0=callerOrigin,1=newKind,2=WrapKind,3=call site
	return newKind(kind, code, msg, base, false, 3)
}

// WrapInternalKind — 5xx ошибка: origin + stack.
func WrapInternalKind(code int, msg string, base error) *ErrorResponse {
	// callerOrigin: 0=callerOrigin,1=newKind,2=WrapInternalKind,3=call site
	return newKind(KindInternal, code, msg, base, true, 3)
}

// New — обычная (как правило 4xx) ошибка с origin, без stack.
func New(code int, msg string) *ErrorResponse {
	// callerOrigin: 0=callerOrigin,1=newKind,2=New,3=call site
	return newKind(KindBadRequest, code, msg, nil, false, 3)
}

// Wrap — ошибка с origin и base (без stack). Подходит для 4xx, где нужен контекст ошибки.
func Wrap(code int, msg string, base error) *ErrorResponse {
	// callerOrigin: 0=callerOrigin,1=newKind,2=Wrap,3=call site
	return newKind(KindBadRequest, code, msg, base, false, 3)
}

// New/Wrap/WrapInternal — единая политика ошибок в проекте:
// - New/Wrap: для ожидаемых 4xx (origin есть, stack не берём).
// - WrapInternal: для внутренних 5xx (origin + stack; stack берём только если base != nil).

// WrapInternal — для “5xx” (внутренних) ошибок: пишет origin + сохраняет стек в ErrBase.
// Использовать вместо ручного &ErrorResponse{Code:..., Err:"...", ErrBase:err}.
func WrapInternal(code int, msg string, base error) *ErrorResponse {
	return WrapInternalKind(code, msg, base)
}

// Удобные конструкторы для “точных” статусов (читается лучше, чем NewKind/WrapKind).
func Unauthorized(code int, msg string) *ErrorResponse {
	return newKind(KindUnauthorized, code, msg, nil, false, 3)
}
func Forbidden(code int, msg string) *ErrorResponse {
	return newKind(KindForbidden, code, msg, nil, false, 3)
}
func NotFound(code int, msg string) *ErrorResponse {
	return newKind(KindNotFound, code, msg, nil, false, 3)
}
func Conflict(code int, msg string) *ErrorResponse {
	return newKind(KindConflict, code, msg, nil, false, 3)
}
func Gone(code int, msg string) *ErrorResponse { return newKind(KindGone, code, msg, nil, false, 3) }

// HTTPError --------------------------------
func (e *HTTPError) Error() string {
	return e.Err + "(Code " + strconv.Itoa(e.Code) + ")\n"
}

type HTTPError struct {
	Code int    `json:"code"`
	Err  string `json:"error"`
}
