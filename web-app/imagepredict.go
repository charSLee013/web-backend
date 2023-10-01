package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"web-app/internal/config"
	"web-app/internal/handler"
	"web-app/internal/svc"

	pb "web-app/proto/public"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var configFile = flag.String("f", "etc/imagepredict.yaml", "the config file")
var redisConfig = flag.String("r", "etc/redis.yaml", "the redis config file")
var milvusConfig = flag.String("m", "etc/milvus.yaml", "the milvus config file")
var ctx = context.Background()

func main() {
	flag.Parse()

	// 加载go-zero配置文件
	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 加载milvus配置文件
	var m config.MilvusConfig
	conf.MustLoad(*milvusConfig, &m)

	// 测试是否能正常连上milvus
	timeout_ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	milvusClient, err := client.NewGrpcClient(timeout_ctx, fmt.Sprintf("%s:%s", m.Host, m.Port))
	if err != nil {
		logx.Errorf("failed to connect to Milvus[%v]:%v", m.Host, err.Error())
		os.Exit(1)
	} else {
		logx.Infof("Connect to milvus server %s success", m.Host)
	}
	err = milvusClient.LoadCollection(timeout_ctx, m.CollectionName, true)
	if err != nil {
		logx.Errorf("failed to load collection[%s]: %v", m.CollectionName, err.Error())
		os.Exit(-1)
	}

	// 加载redis配置文件
	var rds *redis.Redis = nil
	var r redis.RedisConf
	r.PingTimeout = 5 * time.Second
	if (*redisConfig) != "" {
		conf.MustLoad(*redisConfig, &r)
	} else {
		// 尝试从环境变量中获取REDIS信息
		redisHost := os.Getenv("REDIS_HOST")
		redisType := os.Getenv("REDIS_TYPE")
		redisPass := os.Getenv("REDIS_PASS")
		redisTLS := os.Getenv("REDIS_TLS")

		tls, err := strconv.ParseBool(redisTLS)
		if err != nil {
			// 处理解析布尔值的错误
			// fmt.Println("无法解析 Redis TLS 值:", err)
			tls = false // 设置默认值为 false
		}
		r = redis.RedisConf{
			Host:        redisHost,
			Pass:        redisPass,
			Type:        redisType,
			Tls:         tls,
			PingTimeout: 5 * time.Second,
		}
	}

	if r.Type == "" {
		r.Type = "node"
	}

	if r.Host != "" {
		rds = redis.MustNewRedis(r)
		if !rds.PingCtx(timeout_ctx) {
			// 无法ping通则代表redis连接失败
			rds = nil
			logx.Errorf("Cannot ping redis server: %v", r.Host)
		}
		logx.Infof("Connect redis server %s success ", r.Host)
	} else {
		logx.Info("Disable redis cache server")
	}

	// 允许localhost:8080 进行跨域请求
	server := rest.MustNewServer(c.RestConf, rest.WithCustomCors(nil, nil, "http://localhost:8080"))
	defer server.Stop()

	// 连接后端的图像预测功能
	grpc_conn, err := grpc.DialContext(timeout_ctx, c.ImagePredictServer, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logx.Errorf("Cannot connect img predciton serving to %v", c.ImagePredictServer)
	}

	// 创建gRPC客户端
	client := pb.NewImagePredictionClient(grpc_conn)
	ctx := svc.NewServiceContext(c, rds, milvusClient, client)
	logx.Infof("Connect to grpc server %v success", c.ImagePredictServer)

	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
