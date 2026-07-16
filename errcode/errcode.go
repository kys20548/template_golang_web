package errcode

// Code 為 API 回應的業務狀態碼，所有錯誤碼在此統一管理。
// 編碼規則：0 成功；1xxxx 通用錯誤；2xxxx 使用者相關；之後的業務模組依序 3xxxx、4xxxx...
type Code int

const (
	Success Code = 0

	// 通用錯誤 1xxxx
	ErrInternal      Code = 10001
	ErrInvalidParams Code = 10002
	ErrUnauthorized  Code = 10003
	ErrNotFound      Code = 10004
	ErrTimeout       Code = 10005
	ErrNotReady      Code = 10006

	// 使用者相關 2xxxx
	ErrUserNotFound      Code = 20001
	ErrUserExists        Code = 20002
	ErrWrongCredentials  Code = 20003
	ErrTooManyLoginFails Code = 20004

	// 錢包相關 3xxxx（目前只有列表查詢，尚無專屬錯誤碼；新增時從 30001 開始編）
)

var messages = map[Code]string{
	Success:              "success",
	ErrInternal:          "系統內部錯誤",
	ErrInvalidParams:     "參數錯誤",
	ErrUnauthorized:      "未登入或登入已過期",
	ErrNotFound:          "資源不存在",
	ErrTimeout:           "請求處理逾時",
	ErrNotReady:          "服務未就緒",
	ErrUserNotFound:      "使用者不存在",
	ErrUserExists:        "使用者已存在",
	ErrWrongCredentials:  "帳號或密碼錯誤",
	ErrTooManyLoginFails: "登入失敗次數過多，請稍後再試",
}

// Msg 回傳錯誤碼對應的訊息，未定義的碼回傳系統內部錯誤訊息。
func (c Code) Msg() string {
	if msg, ok := messages[c]; ok {
		return msg
	}
	return messages[ErrInternal]
}
