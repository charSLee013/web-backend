package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	DataSource string
	Table      string
	ImagePredictServer string	// 后端图像预测的地址
}


type MilvusConfig struct {
	Host string
	Port string
	CollectionName string
}