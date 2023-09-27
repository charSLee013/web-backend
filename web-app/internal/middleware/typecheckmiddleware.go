package middleware

import (
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"web-app/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// png文件的前8个字节是固定的，分别是：89 50 4E 47 0D 0A 1A 0A，用十六进制表示为 89504e470d0a1a0a
// jpg文件的前2个字节是固定的，分别是：FF D8，用十六进制表示为 ffd8
var fileTypeMap = map[string]string{
	"png": "89504e470d0a1a0a",
	"jpg": "ffd8",
}

type TypeCheckMiddleware struct {
}

func NewTypeCheckMiddleware() *TypeCheckMiddleware {
	return &TypeCheckMiddleware{}
}

func (m *TypeCheckMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 获取文件句柄
		file, _, err := r.FormFile("image")
		if err != nil {
			logx.Error(err)
			http.Error(w, "Internel server error", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// 判断文件类型是否合法
		isVerifly := false
		for _, fc := range fileTypeMap {
			fileHeaderHex, err := getFileHeader(file, len(fc)/2)
			if err != nil {
				logx.Error(err)
				http.Error(w, "Invalid Image Type", http.StatusBadRequest)
			}

			if strings.EqualFold(fileHeaderHex, fc) {
				isVerifly = true
				break
			}
		}

		if !isVerifly {
			// ErrorCode: 1003 图片类型检测不符合
			httpx.WriteJson(w, 200, types.ImageResp{
				Error:     "图片类型错误",
				ErrorCode: 1003,
			})
			return
		}

		// Passthrough to next handler if need
		next(w, r)
	}
}

func getFileHeader(file multipart.File, size int) (string, error) {
	// 读取文件的前8个字节
	header := make([]byte, size)
	_, err := file.Read(header)
	if err != nil && err != io.EOF {
		return "", err
	}

	// 转换成十六进制字符串
	hexHeader := hex.EncodeToString(header)

	// 将文件的偏移量转为0
	_, err = file.Seek(0, 0)
	return hexHeader, nil
}
