package user

import (
	"strconv"

	"yupao-go/internal/middleware"
	"yupao-go/internal/shared/resp"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const sessionKeyUserID = middleware.SessionKeyUserID

type UserHandler struct {
	svc *Service
}

func NewUserHandler(svc *Service) *UserHandler {
	return &UserHandler{svc: svc}
}

// Register
// @Summary  用户注册
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.RegisterParams true "注册参数"
// @Success  200  {object} resp.Response{data=int64}
// @Failure  400  {object} resp.Response
// @Router   /user/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var params RegisterParams
	if err := c.ShouldBindJSON(&params); err != nil {
		resp.RespondBindingError(c, err)
		return
	}
	id, err := h.svc.Register(c.Request.Context(), params)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	resp.RespondOK(c, id)
}

// Login
// @Summary  用户登录
// @Description 用户登录接口，成功后写入 session cookie
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.LoginParams true "登录参数"
// @Success  200  {object} resp.Response{data=user.User}
// @Failure  400  {object} resp.Response
// @Router   /user/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var params LoginParams
	if err := c.ShouldBindJSON(&params); err != nil {
		resp.RespondBindingError(c, err)
		return
	}
	u, err := h.svc.Login(c.Request.Context(), params.UserAccount, params.UserPassword)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	session := sessions.Default(c)
	session.Set(sessionKeyUserID, u.ID)
	session.Save()
	resp.RespondOK(c, u)
}

// Logout
// @Summary  用户注销
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Success  200 {object} resp.Response
// @Router   /user/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	resp.RespondOK(c, nil)
}

// CurrentUser
// @Summary  获取当前登录用户
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Success  200 {object} resp.Response{data=user.User}
// @Failure  401 {object} resp.Response
// @Router   /user/current [get]
func (h *UserHandler) CurrentUser(c *gin.Context) {
	uid, err := middleware.GetLoginUserID(c)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	fresh, err := h.svc.GetByID(c.Request.Context(), uid)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	resp.RespondOK(c, fresh)
}

// SearchByTags
// @Summary  根据标签搜索用户
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Param    tagNameList query    []string true "标签列表"
// @Success  200         {object} resp.Response{data=[]user.User}
// @Failure  400         {object} resp.Response
// @Router   /user/search/tags [get]
func (h *UserHandler) SearchByTags(c *gin.Context) {
	tags := c.QueryArray("tagNameList")
	users, err := h.svc.SearchByTags(c.Request.Context(), tags)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	resp.RespondOK(c, users)
}

// Update
// @Summary  更新用户信息
// @Tags     user
// @Accept   json
// @Produce  json
// @Security SessionAuth
// @Param    body body     user.User true "用户信息"
// @Success  200  {object} resp.Response
// @Failure  400  {object} resp.Response
// @Failure  403  {object} resp.Response
// @Router   /user/update [post]
func (h *UserHandler) Update(c *gin.Context) {
	loginUserID, err := middleware.GetLoginUserID(c)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	var target User
	if err := c.ShouldBindJSON(&target); err != nil {
		resp.RespondBindingError(c, err)
		return
	}
	err = h.svc.Update(c.Request.Context(), target.ID, &target, loginUserID)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	resp.RespondOK(c, nil)
}

// MatchUsers
// @Summary  匹配相似用户
// @Tags     user
// @Produce  json
// @Security SessionAuth
// @Param    num query    int true "推荐数量" minimum(1) maximum(20)
// @Success  200 {object} resp.Response{data=[]user.User}
// @Failure  400 {object} resp.Response
// @Router   /user/match [get]
func (h *UserHandler) MatchUsers(c *gin.Context) {
	numStr := c.Query("num")
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 || num > 20 {
		resp.RespondError(c, resp.NewBizErrorWithDetail(resp.ParamsError, "num 需在 1-20 之间"))
		return
	}
	loginUserID, err := middleware.GetLoginUserID(c)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	loginUser, err := h.svc.GetByID(c.Request.Context(), loginUserID)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	users, err := h.svc.MatchUsers(c.Request.Context(), num, loginUser)
	if err != nil {
		resp.RespondError(c, err)
		return
	}
	resp.RespondOK(c, users)
}
