# 介绍
---

图像预测功能后端

# 编译
---

## 从源代码中编译
请确保安装好 go.18+ 版本，你可以在 [https://go.dev/dl/](https://go.dev/dl/) 选择合适的版本和架构安装
```shell
git clone xxxx
cd web-app/
go build
```

## 构建成容器
请确保安装好 docker 和 docker-compose后，执行下面语句进行构建
```shell
docker build -t golnag-app .
```


# 使用
---

## 参数
* `f`:go-zero 主要配置文件，默认路径是 `etc/imagepredict.yaml`，配置如下
```yaml
Name: ImagePredict 
Host: 0.0.0.0 
Port: 8888          
MaxConns: 1000

# mysql的DSN写法
DataSource: <user>:<password>@tcp(<mysql host>:<mysql port>)/<database>?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai
Table: pixiv_illust
ImagePredictServer: 127.0.0.1:1301
```

* `r`: redis 方面的配置文件，默认路径是 `etc/redis.yaml`，配置如下
```yaml
Host: "127.0.0.1:6379"
Type: "node"
Pass: <redis password>
```

* `m`: milvus 方面的配置，默认路径是 `etc/milvus.yaml`，配置如下
```yaml
Host: "<milvus host>"
Port: "<milvus port>"
CollectionName: "<milvus collection>"
```

# 接口

## 图像预测

### 请求字段
* image: 图片的二进制数据
* md5: 文件的MD5校验码

### 响应字段

* error: 如果没有错误为null,如果有错误则为错误信息
* error_code: 业务码
* contents: 列表，类型为 illust
* tags: 标签列表，类型为 string

下面是 contents.illust 类型的字段
* title: 画集的名称
* illust_id: 画集的ID
* thumb_url: 缩略图URL，表示作品的缩略图地址
* user_name: 用户名称
* profile_img: 用户头像的URL
* rank: 相似度的排名

### 工作流程
1. 检测是否空余的算力，否则返回错误
1. 检测上传图片是否合格
2. 将图片传给<提取图像特征>后端
3. 然后将特征向量传给`milvus`(向量数据库)获取前20个最相似的图片ID
4. 首先查询缓存(`redis`)是否有目标的图片ID数据
5. 如果没有则查询数据库
6. 然后整理返回给前端显示
