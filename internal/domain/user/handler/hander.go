package handler

import (
	"net/http"
	"strconv"

	"yupao-go/internal/core"
	"yupao-go/internal/domain/user"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const sessionKeyUserID = "userID"

type UserHandler struct {
	svc *user.Service
}

func NewUserHandler(svc *user.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

// Register
// @Summary  用户注册
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.RegisterParams true "注册参数"
// @Success  200  {object} core.Response{data=int64}
// @Failure  400  {object} core.Response
// @Router   /user/register [post]
func (h *UserHandler) Register(c *gin.Context) {
	var params user.RegisterParams
	if err := c.ShouldBindJSON(&params); err != nil {
		respondBindingError(c, err)
		return
	}
	id, err := h.svc.Register(c.Request.Context(), params)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, id)
}

// Login
// @Summary  用户登录
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.LoginParams true "登录参数"
// @Success  200  {object} core.Response{data=user.User}
// @Failure  400  {object} core.Response
// @Router   /user/login [post]
func (h *UserHandler) Login(c *gin.Context) {
	var params user.LoginParams
	if err := c.ShouldBindJSON(&params); err != nil {
		respondBindingError(c, err)
		return
	}
	u, err := h.svc.Login(c.Request.Context(), params.UserAccount, params.UserPassword)
	if err != nil {
		respondError(c, err)
		return
	}
	session := sessions.Default(c)
	session.Set(sessionKeyUserID, u.ID)
	session.Save()
	respondOK(c, u)
}

// Logout
// @Summary  用户注销
// @Tags     user
// @Produce  json
// @Success  200 {object} core.Response
// @Router   /user/logout [post]
func (h *UserHandler) Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	respondOK(c, nil)
}

// CurrentUser
// @Summary  获取当前登录用户
// @Tags     user
// @Produce  json
// @Success  200 {object} core.Response{data=user.User}
// @Failure  401 {object} core.Response
// @Router   /user/current [get]
func (h *UserHandler) CurrentUser(c *gin.Context) {
	u, err := h.getLoginUser(c)
	if err != nil {
		respondError(c, err)
		return
	}
	fresh, err := h.svc.GetByID(c.Request.Context(), u.ID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, fresh)
}

// SearchByTags
// @Summary  根据标签搜索用户
// @Tags     user
// @Produce  json
// @Param    tagNameList query    []string true "标签列表"
// @Success  200         {object} core.Response{data=[]user.User}
// @Failure  400         {object} core.Response
// @Router   /user/search/tags [get]
func (h *UserHandler) SearchByTags(c *gin.Context) {
	tags := c.QueryArray("tagNameList")
	users, err := h.svc.SearchByTags(c.Request.Context(), tags)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, users)
}

// Update
// @Summary  更新用户信息
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    body body     user.User true "用户信息"
// @Success  200  {object} core.Response
// @Failure  400  {object} core.Response
// @Failure  403  {object} core.Response
// @Router   /user/update [post]
func (h *UserHandler) Update(c *gin.Context) {
	loginUser, err := h.getLoginUser(c)
	if err != nil {
		respondError(c, err)
		return
	}
	var target user.User
	if err := c.ShouldBindJSON(&target); err != nil {
		respondBindingError(c, err)
		return
	}
	err = h.svc.Update(c.Request.Context(), target.ID, &target, loginUser.ID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, nil)
}

// MatchUsers
// @Summary  匹配相似用户
// @Tags     user
// @Produce  json
// @Param    num query    int true "推荐数量" minimum(1) maximum(20)
// @Success  200 {object} core.Response{data=[]user.User}
// @Failure  400 {object} core.Response
// @Router   /user/match [get]
func (h *UserHandler) MatchUsers(c *gin.Context) {
	numStr := c.Query("num")
	num, err := strconv.Atoi(numStr)
	if err != nil || num <= 0 || num > 20 {
		respondError(c, core.NewBizErrorWithDetail(core.ParamsError, "num 需在 1-20 之间"))
		return
	}
	loginUser, err := h.getLoginUser(c)
	if err != nil {
		respondError(c, err)
		return
	}
	users, err := h.svc.MatchUsers(c.Request.Context(), num, loginUser)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, users)
}

func (h *UserHandler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		uid := session.Get(sessionKeyUserID)
		if uid == nil {
			c.JSON(http.StatusUnauthorized, core.FailWithCode(core.NotLogin, ""))
			c.Abort()
			return
		}
		c.Set(sessionKeyUserID, uid)
		c.Next()
	}
}

func (h *UserHandler) getLoginUser(c *gin.Context) (*user.User, error) {
	uid, exists := c.Get(sessionKeyUserID)
	if !exists {
		return nil, core.NewBizError(core.NotLogin)
	}
	id, ok := uid.(int64)
	if !ok {
		return nil, core.NewBizError(core.NotLogin)
	}
	return h.svc.GetByID(c.Request.Context(), id)
}

func respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, core.OK(data))
}

func respondError(c *gin.Context, err error) {
	c.JSON(core.HTTPCodeFromErr(err), core.Fail(err))
}

func respondBindingError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, core.FailWithCode(core.ParamsError, err.Error()))
}
