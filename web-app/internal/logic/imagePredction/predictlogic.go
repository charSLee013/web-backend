package imagePredction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"web-app/internal/svc"
	"web-app/internal/types"

	"github.com/google/uuid"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "web-app/proto/public"

	"github.com/zeromicro/go-zero/core/logx"
)

type PredictLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
	r      *http.Request
}

// 定义一个缓存的超时时间，单位为秒，这里设置为1天
const CACHEEXPIRE = 24 * 60 * 60

const MODELNAME = "illust2vec"

const COLLECTIONNAME = "pixiv_illust"

func NewPredictLogic(r *http.Request, ctx context.Context, svcCtx *svc.ServiceContext) *PredictLogic {
	return &PredictLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
		r:      r,
	}
}

func (l *PredictLogic) Predict() (resp *types.ImageResp, err error) {
	resp = &types.ImageResp{
		ErrorCode: 0,
	}

	var illustIds []int64
	// 尝试从缓存中获取
	hit, err := l.getPredctionRespFromCache()
	if err == nil {
		illustIds = hit
	}

	// 没有缓存，走一篇正常流程获取图片最相似的前N张图片的ID
	if len(illustIds) < 10 {
		// 将图片转换成特征向量
		vector, err := l.convetImageToVec()
		if err != nil {
			resp.ErrorCode = 1004
			resp.Error = "图像预测失败"
			return resp, nil
		}

		time_ctx, canel := context.WithTimeout(l.ctx, 10*time.Second)
		defer canel()
		searchResults, err := l.predictWithVectorDatabase(time_ctx, vector)
		if err != nil {
			resp.ErrorCode = 1008
			resp.Error = "图像推荐失败"
			return resp, nil
		}

		illustIds = l.getIllustIdColumn(searchResults)
		if len(illustIds) == 0 {
			resp.ErrorCode = 1010
			resp.Error = "没有合适的匹配项"
			return resp, nil
		}
	}

	illusts, err := l.getIllustsFromCacheOrDB(illustIds)
	if err != nil {
		return nil, err
	}

	// 尝试存储iluustIds到缓存中
	go l.storeIllustIdsInCache(illustIds)

	resp.Contents = illusts
	return resp, nil
}

func (l *PredictLogic) storeIllustIdsInCache(illusts []int64) {
	// 将数组转换为字符串
	stringSlice := make([]string, len(illusts))
	for i, num := range illusts {
		stringSlice[i] = strconv.FormatInt(num, 10)
	}
	// 将字符串切片转换为逗号分隔的字符串
	stringData := strings.Join(stringSlice, ",")

	// 存储字符串到Redis中
	md5Sum := l.r.FormValue("md5")
	key := generateResponseCacheKey(md5Sum)
	timeout_ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := l.svcCtx.Redis.SetexCtx(timeout_ctx, key, stringData, 0)
	if err != nil {
		logx.Debug("cannot saved illustIds from response")
		return
	}
}

func (l *PredictLogic) getIllustIdColumn(searchResults []client.SearchResult) []int64 {
	var illustIdColumn *entity.ColumnInt64
	for _, field := range searchResults[0].Fields {
		if field.Name() == "illust_id" {
			c, ok := field.(*entity.ColumnInt64)
			if ok {
				illustIdColumn = c
			}
		}
	}
	return illustIdColumn.Data()
}

// 从缓存中根据图片的MD5值
func (l *PredictLogic) getPredctionRespFromCache() ([]int64, error) {
	md5Sum := l.r.FormValue("md5")
	key := generateResponseCacheKey(md5Sum)
	timeout_ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	result, err := l.svcCtx.Redis.GetCtx(timeout_ctx, key)
	if err != nil {
		return nil, err
	}
	// 将逗号分隔的字符串转换为字符串切片
	stringSlice := strings.Split(result, ",")

	// 将字符串切片转换为[]int64数组
	readNumbers := make([]int64, len(stringSlice))
	for i, str := range stringSlice {
		num, _ := strconv.ParseInt(str, 10, 64)
		readNumbers[i] = num
	}
	return readNumbers, nil
}

func (l *PredictLogic) getIllustsFromCacheOrDB(illustIds []int64) ([]types.Illust, error) {
	var illusts []types.Illust

	for rank, illustId := range illustIds {
		// 首先尝试从缓存中获取作品集信息
		illust, err := l.getIllustsFromCache(illustId)
		if err == nil && illust.IllustID != 0 {
			illust.Rank = int(rank)
			illusts = append(illusts, illust)
			continue
		} else {
			illust, err = l.getIllustsFromDB(illustId)
			if err != nil || illust.IllustID == 0 {
				logx.Errorf("Cannot search %v illust id from database", illustId)
			}
			illust.Rank = int(rank)
			illusts = append(illusts, illust)
		}
		// 异步将作品集信息存储到缓存中，以便下次使用
		go func(illustId int64, illust types.Illust) {
			err = l.storeIllustsInCache(illust, illustId)
			if err != nil {
				// 存储到缓存失败不影响正常返回作品集信息
				logx.Debug("作品集信息存储到缓存失败:", err)
			}
		}(illustId, illust)
	}

	return illusts, nil
}

func (l *PredictLogic) storeIllustsInCache(illust types.Illust, illustId int64) error {
	// 将作品集信息编码为JSON格式
	data, err := json.Marshal(illust)
	if err != nil {
		return err
	}

	// 构造缓存键名
	cacheKey := generateCacheKey(illustId)

	// 存储作品集信息到缓存，设置过期时间
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = l.svcCtx.Redis.SetnxExCtx(ctx, cacheKey, string(data), CACHEEXPIRE)
	if err != nil {
		return err
	}

	return nil
}

func (l *PredictLogic) getIllustsFromDB(illustId int64) (types.Illust, error) {
	timeout_ctx, cancel := context.WithTimeout(l.ctx, 15*time.Second)
	defer cancel()

	illust_resp, err := l.svcCtx.Model.FindOneByIllustId(timeout_ctx, illustId)
	if err != nil {
		return types.Illust{}, err
	}
	illustDetail := types.Illust{
		Title:      illust_resp.Title.String,
		IllustID:   illust_resp.IllustID,
		ThumbURL:   illust_resp.ThumbURL.String,
		UserName:   illust_resp.UserName.String,
		ProfileImg: illust_resp.ProfileImg,
		// Rank:       illustIdColumn.Len() - i,
	}

	return illustDetail, nil
}

func generateCacheKey(illustId int64) string {
	// 根据illust_id生成缓存键名的逻辑
	return fmt.Sprintf("illust_%d", illustId)
}

func generateResponseCacheKey(md5 string) string {
	// 根据图片的md5生成缓存键名的逻辑
	return fmt.Sprintf("predction_md5_%s", md5)
}

func (l *PredictLogic) getIllustsFromCache(illustId int64) (types.Illust, error) {
	var illust types.Illust

	// 从缓存中查询illust_id对应的作品集信息
	cacheKey := generateCacheKey(illustId)
	timeout_ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
	defer cancel()

	data, err := l.svcCtx.Redis.GetCtx(timeout_ctx, cacheKey)
	if err != nil {
		// 缓存获取失败，可能是缓存不存在或发生其他错误
		return illust, err
	}

	if data != "" {
		// 缓存命中，解码作品集信息
		err = json.Unmarshal([]byte(data), &illust)
		if err != nil {
			fmt.Println("解码作品集信息失败:", err)
			// 解码失败则返回错误
			return illust, errors.New("cannot unmarshal data from redis")
		}
	}
	return illust, nil
}

func (l *PredictLogic) convetImageToVec() ([]float32, error) {
	file, handler, err := l.r.FormFile("image")
	if err != nil {
		return []float32{}, err
	}

	defer file.Close()
	md5Sum := l.r.FormValue("md5")

	logx.Infof("upload file: %+v, file size: %d, MIME header: %+v, md5: %s",
		handler.Filename, handler.Size, handler.Header, md5Sum)

	// 构建请求
	request := &pb.ImagePredictionRequest{
		Model: MODELNAME,              // 设置为实际的模型名称
		Id:    int32(uuid.New().ID()), // 设置为实际的请求ID
	}
	logx.Debugf("Make %v convert image to vec", request.Id)

	// 读取请求体内的图片字节
	fileBytes, err := readAllBytes(file)
	if err != nil {
		logx.Errorf("Read bytes from image failed: %v", err.Error())
		return []float32{}, err
	}
	request.Image = fileBytes

	// 设置请求超时时间
	ctx, cancel := context.WithTimeout(l.ctx, 30*time.Second)
	defer cancel()

	// 将图像转成向量
	image_vec_resp, err := l.sendPredictionRequest(ctx, request)
	if err != nil {
		logx.Errorf("调用Predict方法失败: %v", err)
		return []float32{}, err
	}

	return image_vec_resp.Vector, nil
}

func readAllBytes(file multipart.File) ([]byte, error) {
	buffer := bytes.NewBuffer(nil)
	_, err := io.Copy(buffer, file)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (l *PredictLogic) sendPredictionRequest(ctx context.Context, request *pb.ImagePredictionRequest) (*pb.ImageVectorResponse, error) {
	conn, _ := grpc.Dial("127.0.0.1:1301", grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()
	client := pb.NewImagePredictionClient(conn)

	stream, err := client.Predict(ctx)
	if err != nil {
		return nil, fmt.Errorf("调用Predict方法失败：%v", err)
	}

	err = stream.Send(request)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败：%v", err)
	} else {
		logx.Debugf("发送图像转换成向量请求编号: %v", request.Id)
	}
	stream.CloseSend()

	responseCh := make(chan *pb.ImageVectorResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		response, err := stream.Recv()
		if err == io.EOF {
			errCh <- nil
		}
		if err != nil {
			errCh <- fmt.Errorf("接收响应失败：%v", err)
			return
		}
		responseCh <- response
	}()

	select {
	case response := <-responseCh:
		return response, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("等待响应超时")
	}
}
func (l *PredictLogic) predictWithVectorDatabase(ctx context.Context, featureVector []float32) ([]client.SearchResult, error) {
	// 向量数据库预测逻辑
	// 返回预测的实体（Entity）和错误（error）

	// 这里选择IVF_FLAT类型，必须跟创建集合的类型相同
	sp, _ := entity.NewIndexIvfFlatSearchParam( // NewIndex*SearchParam func
		512, // npore 必须比建立集合的nlist小或等于，表示搜索的范围
	)

	opt := client.SearchQueryOptionFunc(func(option *client.SearchQueryOption) {
		option.Limit = 20
		// option.Offset = 0
		option.ConsistencyLevel = entity.ClStrong
		option.IgnoreGrowing = false
	})

	searchResult, err := l.svcCtx.Milvus.Search(
		ctx,
		COLLECTIONNAME,
		[]string{},
		"",
		[]string{"illust_id", "meta"},
		[]entity.Vector{entity.FloatVector(featureVector)},
		"vector",
		entity.COSINE,
		20,
		sp,
		opt,
	)
	if err != nil {
		logx.Errorf("fail to search collection:%v", err.Error())
		return []client.SearchResult{}, err
	}
	return searchResult, nil
}
