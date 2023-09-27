package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"web-app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type MD5CheckMiddleware struct {
}

func NewMD5CheckMiddleware() *MD5CheckMiddleware {
	return &MD5CheckMiddleware{}
}

func (m *MD5CheckMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 获取请求体中的图片和md5参数
		f, fh, err := r.FormFile("image")
		if err != nil {
			logx.Error("formfile error", err)
			httpx.Error(w, err)
			return
		}
		defer f.Close()
		md5Sum := r.FormValue("md5")

		// 判断图片的二进制数据大小是否为0
		if fh.Size == 0 {
			// ErrorCode: 1001 接收的图片为空
			httpx.WriteJson(w, 200, types.ImageResp{
				Error:     "图片为空",
				ErrorCode: 1001,
			})
			return
		}

		// 计算图片的md5值
		hash := md5.New()
		_, err = io.Copy(hash, f)
		if err != nil {
			logx.Error("calculator image md5 failed", err)
			httpx.Error(w, err)
			return
		}
		calculatedMD5 := hex.EncodeToString(hash.Sum(nil))

		// 判断图片的md5值是否与参数一致
		if !strings.EqualFold(md5Sum, calculatedMD5) {
			// ErrorCode: 1002 MD5校验不通过
			httpx.WriteJson(w, 200, types.ImageResp{
				Error:     "MD5校验不通过",
				ErrorCode: 1002,
			})
			return
		}

		// 如果检测通过，继续执行后续的处理逻辑
		next(w, r)
	}
}
