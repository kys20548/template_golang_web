// hasPerm 檢查登入者（/me 回傳的 AuthUser）是否具備某權限；'*' 為萬用。
export function hasPerm(user, code) {
  return !!user?.permissions?.some((p) => p === '*' || p === code)
}
