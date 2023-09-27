package svc

import (
	"web-app/internal/config"
	"web-app/internal/middleware"

	"web-app/model"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"

	pb "web-app/proto/public"
)

type ServiceContext struct {
	Config        config.Config
	Model         model.PixivIllustModel
	Milvus        client.Client
	Redis         *redis.Redis // 创建一个redis客户端实例，并存储在服务上下文中
	PredctionGRPC pb.ImagePredictionClient
	MD5Check      rest.Middleware
	TypeCheck     rest.Middleware
}

func NewServiceContext(c config.Config, redis *redis.Redis, milvus client.Client, grpc pb.ImagePredictionClient) *ServiceContext {
	return &ServiceContext{
		Config:        c,
		Model:         model.NewPixivIllustModel(sqlx.NewMysql(c.DataSource)),
		Milvus:        milvus,
		Redis:         redis,
		PredctionGRPC: grpc,
		MD5Check:      middleware.NewMD5CheckMiddleware().Handle,
		TypeCheck:     middleware.NewTypeCheckMiddleware().Handle,
	}
}
