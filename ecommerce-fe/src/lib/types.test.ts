import { describe, expect, it } from "vitest"
import type { UpdateProfileRequest } from "./types"

describe("UpdateProfileRequest", () => {
  it("models PATCH subsets where every mutable field is optional", () => {
    const phoneOnly: UpdateProfileRequest = { phone: "0901234567" }
    const locationOnly: UpdateProfileRequest = { provinceCode: "79", wardCode: "26740" }
    const genderClear: UpdateProfileRequest = { gender: "" }
    expect(phoneOnly.phone).toBe("0901234567")
    expect(locationOnly.wardCode).toBe("26740")
    expect(genderClear.gender).toBe("")
  })
})
