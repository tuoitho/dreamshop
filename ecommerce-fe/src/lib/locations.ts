import { http, toClientError } from "./api"
import type { LocationOption } from "./types"

export async function fetchProvinces(): Promise<LocationOption[]> {
  try {
    const { data } = await http.get<LocationOption[]>("/locations/provinces")
    return data
  } catch (error) {
    throw toClientError(error)
  }
}

export async function fetchWards(provinceCode: string): Promise<LocationOption[]> {
  try {
    const { data } = await http.get<LocationOption[]>(
      `/locations/provinces/${encodeURIComponent(provinceCode)}/wards`
    )
    return data
  } catch (error) {
    throw toClientError(error)
  }
}
