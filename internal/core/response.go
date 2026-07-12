package core

import "errors"

type Response struct {
	Code        int    `json:"code"`
	Data        any    `json:"data"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
}

func OK(data any) *Response {
	return &Response{
		Code:    Success.Biz,
		Data:    data,
		Message: Success.Message,
	}
}

func Fail(err error) *Response {
	var bizErr *BizError
	if errors.As(err, &bizErr) {
		return &Response{
			Code:    bizErr.BizCode(),
			Message: bizErr.Error(),
		}
	}
	return &Response{
		Code:    SystemError.Biz,
		Message: SystemError.Message,
	}
}

func FailWithCode(code Code, detail string) *Response {
	msg := code.Message
	if detail != "" {
		msg = detail
	}
	return &Response{
		Code:    code.Biz,
		Message: msg,
	}
}

func HTTPCodeFromErr(err error) int {
	var bizErr *BizError
	if errors.As(err, &bizErr) {
		return bizErr.HTTPCode()
	}
	return SystemError.HTTP
}
