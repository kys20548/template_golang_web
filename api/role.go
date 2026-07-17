package api

import (
	"time"

	"github.com/gin-gonic/gin"
)

// roleResponse 為角色的對外回應結構，帶該角色的 permission codes。
type roleResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
}

// listRoles 查詢全部角色與各自的權限清單（唯讀；角色異動用 migration/SQL 管）。
// 角色數量小，不分頁。
//
// @Summary  角色列表（含權限）
// @Tags     admin-user
// @Produce  json
// @Security TokenAuth
// @Success  200 {object} Response{data=[]roleResponse}
// @Router   /roles [get]
func (server *Server) listRoles(ctx *gin.Context) {
	roles, err := server.store.ListRoles(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}

	rolePerms, err := server.store.ListRolePermissions(ctx)
	if err != nil {
		failInternal(ctx, err)
		return
	}
	permsByRole := make(map[int64][]string)
	for _, rp := range rolePerms {
		permsByRole[rp.RoleID] = append(permsByRole[rp.RoleID], rp.Code)
	}

	list := make([]roleResponse, 0, len(roles))
	for _, role := range roles {
		perms := permsByRole[role.ID]
		if perms == nil {
			perms = []string{}
		}
		list = append(list, roleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			Permissions: perms,
			CreatedAt:   role.CreatedAt,
		})
	}

	ok(ctx, list)
}
