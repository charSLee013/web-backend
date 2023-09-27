package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ PixivIllustModel = (*customPixivIllustModel)(nil)

type (
	// PixivIllustModel is an interface to be customized, add more methods here,
	// and implement the added methods in customPixivIllustModel.
	PixivIllustModel interface {
		pixivIllustModel
	}

	customPixivIllustModel struct {
		*defaultPixivIllustModel
	}
)

// NewPixivIllustModel returns a model for the database table.
func NewPixivIllustModel(conn sqlx.SqlConn) PixivIllustModel {
	return &customPixivIllustModel{
		defaultPixivIllustModel: newPixivIllustModel(conn),
	}
}
