package userhttp

import (
	"strconv"

	"yupao-go/internal/httpapi/middleware"
	"yupao-go/internal/module/user"
	"yupao-go/internal/pkg/logger"
	"yupao-go/internal/pkg/response"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Handler 用户 HTTP 接口层，依赖 user.Service，不包含业务逻辑。
type Handler struct {
	svc *user.Service
}

// NewHandler 构造用户 HTTP Handler。
func NewHandler(svc *user.Service) *Handler {
	return &Handler{svc: svc}
}

// Register
// @Summary  用户注册
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.RegisterParams true "注册参数"
// @Success  200  {object} response.Response{data=int64}
// @Failure  400  {object} response.Response
// @Router   /user/register [post]
func (h *Handler) Register(c *gin.Context) {
	var params user.RegisterParams
	if err := c.ShouldBindJSON(&params); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	id, err := h.svc.Register(c.Request.Context(), params)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, id)
}

// Login
// @Summary  用户登录
// @Description 用户登录接口，成功后写入 session cookie
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.LoginParams true "登录参数"
// @Success  200  {object} response.Response{data=user.User}
// @Failure  400  {object} response.Response
// @Router   /user/login [post]
func (h *Handler) Login(c *gin.Context) {
	var params user.LoginParams
	if err := c.ShouldBindJSON(&params); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	u, err := h.svc.Login(c.Request.Context(), params.UserAccount, params.UserPassword)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	session := sessions.Default(c)
	session.Set(middleware.SessionKeyUserID, u.ID)
	_ = session.Save()
	response.RespondOK(c, u)
}

// Logout
// @Summary  用户注销
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Success  200 {object} response.Response
// @Router   /user/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	uid := session.Get(middleware.SessionKeyUserID)
	session.Clear()
	_ = session.Save()
	logger.Module("user").Info("user logged out",
		logger.FieldPurpose, logger.PurposeAudit,
		logger.FieldEvent, "user.logout",
		"user_id", uid,
	)
	response.RespondOK(c, nil)
}

// CurrentUser
// @Summary  获取当前登录用户
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Success  200 {object} response.Response{data=user.User}
// @Failure  401 {object} response.Response
// @Router   /user/current [get]
func (h *Handler) CurrentUser(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	fresh, err := h.svc.GetByID(c.Request.Context(), uid)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, fresh)
}

// SearchByTags
// @Summary  根据标签搜索用户
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Param    tagNameList query    []string true "标签列表"
// @Success  200         {object} response.Response{data=[]user.User}
// @Failure  400         {object} response.Response
// @Router   /user/search/tags [get]
func (h *Handler) SearchByTags(c *gin.Context) {
	tags := c.QueryArray("tagNameList")

	users, err := h.svc.SearchByTags(c.Request.Context(), tags)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, users)
}

// Update
// @Summary  更新用户信息
// @Tags     user
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body     user.User true "用户信息"
// @Success  200  {object} response.Response
// @Failure  400  {object} response.Response
// @Failure  403  {object} response.Response
// @Router   /user/update [post]
func (h *Handler) Update(c *gin.Context) {
	loginUserID, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	var target user.User
	if err := c.ShouldBindJSON(&target); err != nil {
		response.RespondBindingError(c, err)
		return
	}
	err = h.svc.Update(c.Request.Context(), target.ID, &target, loginUserID)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, nil)
}

// MatchUsers
// @Summary  匹配相似用户
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Param    num query    int true "推荐数量" minimum(1) maximum(20)
// @Success  200 {object} response.Response{data=[]user.User}
// @Failure  400 {object} response.Response
// @Router   /user/match [get]
func (h *Handler) MatchUsers(c *gin.Context) {
	numStr := c.Query("num")
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 || num > 20 {
		response.RespondError(c, response.NewBizErrorWithDetail(response.ParamsError, "num 需在 1-20 之间"))
		return
	}
	loginUserID, err := middleware.GetLoginUserID(c)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	loginUser, err := h.svc.GetByID(c.Request.Context(), loginUserID)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	users, err := h.svc.MatchUsers(c.Request.Context(), loginUser, num)
	if err != nil {
		response.RespondError(c, err)
		return
	}
	response.RespondOK(c, users)
}
