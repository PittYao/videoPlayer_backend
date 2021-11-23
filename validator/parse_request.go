package zh_validator

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"net/http"
	http2 "videoPlayer/http"
)

func ParseRequest(c *gin.Context, request interface{}) error {
	err := c.ShouldBind(request)

	if err != nil {
		switch err.(type) {
		case *json.UnmarshalTypeError:
			log.Warn().Msgf("参数类型异常")
			c.JSON(http.StatusBadRequest, http2.Error("参数类型异常"))
		default:
			translate := Translate(err.(validator.ValidationErrors))
			log.Warn().Msgf("缺少请求参数：", translate)
			c.JSON(http.StatusBadRequest, http2.Error(translate))
		}
		return err
	}
	return nil
}
