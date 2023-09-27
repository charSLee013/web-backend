package imagePredction

import (
	"net/http"

	"web-app/internal/logic/imagePredction"
	"web-app/internal/svc"

	"github.com/zeromicro/go-zero/rest/httpx"
)

func PredictHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := imagePredction.NewPredictLogic(r, r.Context(), svcCtx)
		resp, err := l.Predict()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
