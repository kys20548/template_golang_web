import { getToken } from '../auth/session'

const BASE_URL = import.meta.env.VITE_API_BASE_URL

export async function request(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...options.headers }
  const token = getToken()
  if (token) headers.token = token

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers })
  const body = await res.json()

  if (body.code !== 0) {
    throw new Error(body.msg || `request failed (code ${body.code})`)
  }
  return body.data
}
