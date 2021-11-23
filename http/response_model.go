package http

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(message string, data interface{}) Response {
	var response Response
	response.Code = 200
	response.Message = message
	response.Data = data
	return response
}

func Error(message string) Response {
	var response Response
	response.Code = 500
	response.Message = message
	response.Data = ""
	return response
}

func ErrorWithData(message string, data interface{}) Response {
	var response Response
	response.Code = 500
	response.Message = message
	response.Data = data
	return response
}
