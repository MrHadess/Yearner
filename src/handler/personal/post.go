// Copyright 2019 HenryYee.
//
// Licensed under the AGPL, Version 3.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.gnu.org/licenses/agpl-3.0.en.html
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package personal

import (
	"Yearning-go/src/handler/common"
	"Yearning-go/src/handler/manage/tpl"
	"Yearning-go/src/lib"
	"Yearning-go/src/model"
	"encoding/json"
	"fmt"
	"github.com/cookieY/yee"
	"net/http"
	"strings"
	"time"
)

func Post(c yee.Context) (err error) {
	switch c.Params("tp") {
	case "post":
		return sqlOrderPost(c)
	case "edit":
		return PersonalUserEdit(c)
	case "copyOrderByIDC":
		return createByIDC(c)
	}
	return err
}

func sqlOrderPost(c yee.Context) (err error) {

	u := new(model.CoreSqlOrder)
	user := new(lib.Token).JwtParse(c)
	if err = c.Bind(u); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, common.ERR_REQ_BIND)
	}
	length, err := wrapperPostOrderInfo(u, c)
	if err != nil {
		return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(err))
	}
	u.ID = 0
	model.DB().Create(u)
	model.DB().Create(&model.CoreWorkflowDetail{
		WorkId:   u.WorkId,
		Username: user.Username,
		Action:   "已提交",
		Time:     time.Now().Format("2006-01-02 15:04"),
	})

	lib.MessagePush(u.WorkId, 2, "")

	if u.Type == lib.DML {
		CallAutoTask(u, length)
	}

	return c.JSON(http.StatusOK, common.SuccessPayLoadToMessage(ORDER_POST_SUCCESS))
}

func wrapperPostOrderInfo(order *model.CoreSqlOrder, y yee.Context) (length int, err error) {
	var from model.CoreWorkflowTpl
	var flowId model.CoreDataSource
	var step []tpl.Tpl
	model.DB().Model(model.CoreDataSource{}).Where("source_id = ?", order.SourceId).First(&flowId)
	model.DB().Model(model.CoreWorkflowTpl{}).Where("id =?", flowId.FlowID).Find(&from)
	err = json.Unmarshal(from.Steps, &step)
	if err != nil || len(step) < 2 {
		y.Logger().Error(err)
		return 0, err
	}
	user := new(lib.Token).JwtParse(y)
	w := lib.GenWorkid()
	if order.Source == "" {
		order.Source = flowId.Source
	}
	if order.IDC == "" {
		order.IDC = flowId.IDC
	}
	order.WorkId = w
	order.Username = user.Username
	order.RealName = user.RealName
	order.Date = time.Now().Format("2006-01-02 15:04")
	order.Status = 2
	order.Time = time.Now().Format("2006-01-02")
	order.CurrentStep = 1
	order.Assigned = strings.Join(step[1].Auditor, ",")
	order.Relevant = lib.JsonStringify(order.Relevant)
	return len(step), nil
}

// Order id must be nut nil,and order need execuster
func createByIDC(c yee.Context) error {
	reqOrder := new(model.CoreSqlOrder)
	user := new(lib.Token).JwtParse(c)
	if err := c.Bind(reqOrder); err != nil {
		c.Logger().Error(err.Error())
		return c.JSON(http.StatusOK, common.ERR_REQ_BIND)
	}

	// Check order status is done!
	var oldOrder *model.CoreSqlOrder
	model.DB().Model(model.CoreSqlOrder{}).Where("work_id = ?", reqOrder.WorkId).Find(&oldOrder)
	if oldOrder == nil {
		return c.JSON(http.StatusOK, common.SuccessPayLoadToMessage("未找到对应的工单"))
	}
	if oldOrder.Status != 1 {
		return c.JSON(http.StatusOK, common.SuccessPayLoadToMessage("该任务尚未结束"))
	}

	// check value empty
	if reqOrder.DataBase == "" || reqOrder.SourceId == "" {
		return c.JSON(http.StatusOK, common.SuccessPayLoadToMessage("请求参数有误"))
	}

	// Set up new order param by old

	length, err := wrapperPostByOldOrder(oldOrder, reqOrder, c)
	if err != nil {
		return c.JSON(http.StatusOK, common.ERR_COMMON_MESSAGE(err))
	}
	reqOrder.ID = 0
	model.DB().Create(reqOrder)
	model.DB().Create(&model.CoreWorkflowDetail{
		WorkId:   reqOrder.WorkId,
		Username: user.Username,
		Action:   "已提交",
		Time:     time.Now().Format("2006-01-02 15:04"),
	})

	lib.MessagePush(reqOrder.WorkId, 2, "")

	if reqOrder.Type == lib.DML {
		CallAutoTask(reqOrder, length)
	}

	return c.JSON(http.StatusOK, common.SuccessPayLoadToMessage(ORDER_POST_SUCCESS))
}

// order - Data source by request. need setting fields:[SourceId, DataBase, Table]
func wrapperPostByOldOrder(oldOrder *model.CoreSqlOrder, order *model.CoreSqlOrder, y yee.Context) (length int, err error) {
	var from model.CoreWorkflowTpl
	var flowId model.CoreDataSource
	var step []tpl.Tpl
	model.DB().Model(model.CoreDataSource{}).Where("source_id = ?", order.SourceId).First(&flowId)
	model.DB().Model(model.CoreWorkflowTpl{}).Where("id =?", flowId.FlowID).Find(&from)
	err = json.Unmarshal(from.Steps, &step)
	if err != nil || len(step) < 2 {
		y.Logger().Error(err)
		return 0, err
	}
	user := new(lib.Token).JwtParse(y)
	w := lib.GenWorkid()
	if order.Source == "" {
		order.Source = flowId.Source
	}
	if order.IDC == "" {
		order.IDC = flowId.IDC
	}
	// Set new order used [SourceId, DataBase, Table] --- Check not empty
	// Set new order data by old
	order.Type = oldOrder.Type
	order.Backup = oldOrder.Backup
	order.IDC = oldOrder.IDC
	order.SQL = oldOrder.SQL
	order.Text = fmt.Sprintf("%s[Order by id %v]", order.Text, oldOrder.ID)
	// Set new order field data
	order.WorkId = w
	order.Username = user.Username
	order.RealName = user.RealName
	order.Date = time.Now().Format("2006-01-02 15:04")
	order.Status = 2 // status is not exec
	order.Time = time.Now().Format("2006-01-02")
	order.CurrentStep = 1
	order.Assigned = strings.Join(step[1].Auditor, ",")
	order.Relevant = lib.JsonStringify(order.Relevant)
	return len(step), nil
}
