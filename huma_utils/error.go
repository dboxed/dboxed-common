package huma_utils

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/dboxed/dboxed-common/util"
)

func InitHumaErrorOverride() {
	orig := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		if status == http.StatusInternalServerError {
			for _, err := range errs {
				if util.IsSqlNotFoundError(err) {
					status = http.StatusNotFound
				} else if util.IsSqlUniqueConstraintViolationError(err) {
					status = http.StatusConflict
				}
			}
		}
		return orig(status, msg, errs...)
	}
}
