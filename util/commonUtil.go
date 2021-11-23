package util

import (
	uuid "github.com/satori/go.uuid"
	"net/http"
	"sync"
)

// 响应实体
type ResponseModel struct {
	Code    int64       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

var FFmpegMutex = sync.RWMutex{}

func Success(message string, data interface{}) ResponseModel {
	var response ResponseModel
	response.Code = http.StatusOK
	response.Message = message
	response.Data = data
	return response
}

func Error(message string) ResponseModel {
	var response ResponseModel
	response.Code = http.StatusInternalServerError
	response.Message = message
	response.Data = nil
	return response
}

func RandomStringId() string {
	//int63 := time.Now().UnixNano()
	//return strconv.FormatInt(int63, 10)
	uuid := uuid.NewV4()
	return uuid.String()
}
