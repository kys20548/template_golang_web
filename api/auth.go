package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/errcode"
	"github.com/rs/zerolog/log"
)

type loginRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
}

type loginResponse struct {
	Token string  `json:"token"`
	User  db.User `json:"user"`
}

// login 範例登入：查詢使用者後產生 token，把 user 資訊存入 Redis。
// 實務上這裡應該再加上密碼驗證。
func (server *Server) login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		fail(ctx, http.StatusBadRequest, errcode.ErrInvalidParams, err)
		return
	}

	user, err := server.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fail(ctx, http.StatusNotFound, errcode.ErrUserNotFound, nil)
			return
		}
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	token := uuid.NewString()
	authUser := AuthUser{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
	}
	payload, err := json.Marshal(authUser)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	err = server.cache.Set(ctx, sessionKey(token), string(payload), server.config.TokenDuration)
	if err != nil {
		fail(ctx, http.StatusInternalServerError, errcode.ErrInternal, err)
		return
	}

	ok(ctx, loginResponse{Token: token, User: user})
}

// me 示範從 context 取出驗證層放入的登入者資訊。
func (server *Server) me(ctx *gin.Context) {
	user := getAuthUser(ctx)
	log.Info().
		Int64("user_id", user.UserID).
		Str("username", user.Username).
		Msg("get current user")

	ok(ctx, user)
}
