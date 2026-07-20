package teamhttp

import (
	"strconv"

	"yupao-go/internal/httpapi/middleware"
	"yupao-go/internal/module/team"
	"yupao-go/internal/module/user"
	"yupao-go/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// Handler 队伍 HTTP 接口。
type Handler struct {
	svc     *team.Service
	usersvc *user.Service
}

// NewHandler 构造队伍 Handler；users 用于解析当前用户是否管理员。
func NewHandler(svc *team.Service, usersvc *user.Service) *Handler {
	return &Handler{svc: svc, usersvc: usersvc}
}

// Add
// @Summary  创建队伍
// @Tags     team
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body team.AddParams true "创建参数"
// @Success  200 {object} response.Response{data=int64}
// @Router   /team/add [post]
func (h *Handler) Add(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var p team.AddParams
	if err := c.ShouldBindJSON(&p); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	id, err := h.svc.Add(c.Request.Context(), p, uid)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, id)
}

// Update
// @Summary  更新队伍
// @Tags     team
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body team.UpdateParams true "更新参数"
// @Success  200 {object} response.Response{data=bool}
// @Router   /team/update [post]
func (h *Handler) Update(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var p team.UpdateParams
	if err := c.ShouldBindJSON(&p); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	isAdmin, err := h.resolveAdmin(c, uid)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	if err := h.svc.Update(c.Request.Context(), p, uid, isAdmin); err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, true)
}

// Get
// @Summary  根据 ID 获取队伍
// @Tags     team
// @Produce  json
// @Param    id query int true "队伍 ID"
// @Success  200 {object} response.Response{data=team.Team}
// @Router   /team/get [get]
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Query("id"), 10, 64)
	if err != nil || id <= 0 {
		response.RespondError(c, response.NewBizError(response.ParamsError))
		return
	}
	uid, isAdmin := h.optionalLogin(c)
	t, err := h.svc.GetByID(c.Request.Context(), id, uid, isAdmin)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, t)
}

// List
// @Summary  队伍列表
// @Tags     team
// @Produce  json
// @Param    searchText query string false "关键词"
// @Param    status query int false "状态"
// @Param    name query string false "名称"
// @Success  200 {object} response.Response{data=[]team.TeamUserVO}
// @Router   /team/list [get]
func (h *Handler) List(c *gin.Context) {
	var q team.QueryParams
	if err := c.ShouldBindQuery(&q); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	uid, isAdmin := h.optionalLogin(c)
	list, err := h.svc.List(c.Request.Context(), q, uid, isAdmin)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, list)
}

// ListPage
// @Summary  队伍分页列表
// @Tags     team
// @Produce  json
// @Param    pageNum query int false "页码"
// @Param    pageSize query int false "页大小"
// @Success  200 {object} response.Response
// @Router   /team/list/page [get]
func (h *Handler) ListPage(c *gin.Context) {
	var q team.QueryParams
	if err := c.ShouldBindQuery(&q); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	uid, isAdmin := h.optionalLogin(c)
	page, err := h.svc.ListPage(c.Request.Context(), q, uid, isAdmin)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, page)
}

// Join
// @Summary  加入队伍
// @Tags     team
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body team.JoinParams true "加入参数"
// @Success  200 {object} response.Response{data=bool}
// @Router   /team/join [post]
func (h *Handler) Join(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var p team.JoinParams
	if err := c.ShouldBindJSON(&p); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	if err := h.svc.Join(c.Request.Context(), p, uid); err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, true)
}

// Quit
// @Summary  退出队伍
// @Tags     team
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body team.QuitParams true "退出参数"
// @Success  200 {object} response.Response{data=bool}
// @Router   /team/quit [post]
func (h *Handler) Quit(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var p team.QuitParams
	if err := c.ShouldBindJSON(&p); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	if err := h.svc.Quit(c.Request.Context(), p, uid); err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, true)
}

// Delete
// @Summary  解散队伍
// @Tags     team
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body team.DeleteParams true "解散参数"
// @Success  200 {object} response.Response{data=bool}
// @Router   /team/delete [post]
func (h *Handler) Delete(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var p team.DeleteParams
	if err := c.ShouldBindJSON(&p); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	if err := h.svc.Delete(c.Request.Context(), p.ID, uid); err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, true)
}

// ListMyCreate
// @Summary  我创建的队伍
// @Tags     team
// @Produce  json
// @Security SessionAuth
// @Success  200 {object} response.Response{data=[]team.TeamUserVO}
// @Router   /team/list/my/create [get]
func (h *Handler) ListMyCreate(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var q team.MyCreateQueryParams
	if err := c.ShouldBindQuery(&q); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	list, err := h.svc.ListMyCreate(c.Request.Context(), q, uid)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, list)
}

// ListMyJoin
// @Summary  我加入的队伍
// @Tags     team
// @Produce  json
// @Security SessionAuth
// @Success  200 {object} response.Response{data=[]team.TeamUserVO}
// @Router   /team/list/my/join [get]
func (h *Handler) ListMyJoin(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var q team.MyJoinQueryParams
	if err := c.ShouldBindQuery(&q); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	list, err := h.svc.ListMyJoin(c.Request.Context(), q, uid)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, list)
}

func (h *Handler) resolveAdmin(c *gin.Context, uid int64) (bool, error) {
	u, err := h.usersvc.GetByID(c.Request.Context(), uid)
	if err != nil {
		return false, err
	}
	return h.usersvc.IsAdmin(u), nil
}

// optionalLogin 尝试读取登录用户；未登录返回 0,false。
func (h *Handler) optionalLogin(c *gin.Context) (int64, bool) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		return 0, false
	}
	isAdmin, err := h.resolveAdmin(c, uid)
	if err != nil {
		return uid, false
	}
	return uid, isAdmin
}
