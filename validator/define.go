package zh_validator

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	zh_translations "github.com/go-playground/validator/v10/translations/zh"
)

var (
	v     *validator.Validate
	trans ut.Translator
)

// 参数校验

func InitValidator() {
	// 中文翻译
	zh := zh.New()
	uni := ut.New(zh, zh)
	trans, _ = uni.GetTranslator("zh")

	v, ok := binding.Validator.Engine().(*validator.Validate)
	if ok {
		// 验证器注册翻译器
		zh_translations.RegisterDefaultTranslations(v, trans)
		// 自定义验证方法
		v.RegisterValidation("checkMobile", checkMobile)
	}
}

func Translate(errs validator.ValidationErrors) string {
	var errList []string
	for _, e := range errs {
		// can translate each error one at a time.
		errList = append(errList, e.Translate(trans))
	}
	return strings.Join(errList, "|")
}

func checkMobile(fl validator.FieldLevel) bool {
	mobile := strconv.Itoa(int(fl.Field().Uint()))
	if len(mobile) != 11 {
		return false
	}
	return true
}
