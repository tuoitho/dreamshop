type AccessTokenListener = (token: string | null) => void

let accessToken: string | null = null
let refreshInFlight: Promise<string> | null = null
let sessionEpoch = 0
const listeners = new Set<AccessTokenListener>()

function notifyAccessTokenListeners(nextToken: string | null): void {
  for (const listener of listeners) {
    listener(nextToken)
  }
}

export function getAccessToken(): string | null {
  return accessToken
}

export function getSessionEpoch(): number {
  return sessionEpoch
}

export function advanceSessionEpoch(): number {
  sessionEpoch += 1
  refreshInFlight = null
  return sessionEpoch
}

export function setAccessToken(token: string): void {
  if (accessToken === token) return
  accessToken = token
  notifyAccessTokenListeners(accessToken)
}

export function clearAccessToken(): void {
  sessionEpoch += 1
  refreshInFlight = null
  if (accessToken === null) return
  accessToken = null
  notifyAccessTokenListeners(null)
}

export function subscribeAccessToken(listener: AccessTokenListener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function runSingleFlightRefresh(refresh: () => Promise<string>): Promise<string> {
  if (!refreshInFlight) {
    const startEpoch = sessionEpoch
    const attempt = refresh()
      .then((token) => {
        if (sessionEpoch !== startEpoch) {
          throw new Error("Session changed during token refresh")
        }
        return token
      })
      .finally(() => {
        if (refreshInFlight === attempt) {
          refreshInFlight = null
        }
      })
    refreshInFlight = attempt
  }
  return refreshInFlight
}
