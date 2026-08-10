let accessToken: string | null = null
let refreshInFlight: Promise<string> | null = null

export function getAccessToken(): string | null {
  return accessToken
}

export function setAccessToken(token: string): void {
  accessToken = token
}

export function clearAccessToken(): void {
  accessToken = null
}

export function runSingleFlightRefresh(refresh: () => Promise<string>): Promise<string> {
  if (!refreshInFlight) {
    refreshInFlight = refresh().finally(() => {
      refreshInFlight = null
    })
  }
  return refreshInFlight
}
