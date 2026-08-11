import type { ComponentProps } from "react"
import { expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { SearchableSelect } from "./searchable-select"

const OPTIONS = [
  { code: "01", name: "Hà Nội" },
  { code: "48", name: "Đà Nẵng" },
  { code: "79", name: "TP. Hồ Chí Minh" },
]

function renderSelect(props: Partial<ComponentProps<typeof SearchableSelect>> = {}) {
  const onValueChange = vi.fn()
  render(
    <SearchableSelect
      value=""
      placeholder="Chọn tỉnh/thành phố"
      options={OPTIONS}
      onValueChange={onValueChange}
      searchLabel="Tìm tỉnh/thành phố"
      {...props}
    />
  )
  return { onValueChange }
}

it("exposes aria-expanded and links the trigger and search input to the listbox", async () => {
  renderSelect()
  const user = userEvent.setup()

  const trigger = screen.getByRole("button", { name: "Chọn tỉnh/thành phố" })
  expect(trigger).toHaveAttribute("aria-haspopup", "listbox")
  expect(trigger).toHaveAttribute("aria-expanded", "false")

  await user.click(trigger)

  expect(trigger).toHaveAttribute("aria-expanded", "true")
  const input = screen.getByRole("combobox")
  const listbox = screen.getByRole("listbox")
  expect(trigger).toHaveAttribute("aria-controls", listbox.id)
  expect(input).toHaveAttribute("aria-controls", listbox.id)
})

it("navigates filtered options with ArrowDown/ArrowUp and selects the active option with Enter", async () => {
  const { onValueChange } = renderSelect()
  const user = userEvent.setup()

  await user.click(screen.getByRole("button", { name: "Chọn tỉnh/thành phố" }))
  const input = screen.getByRole("combobox")
  const options = () => screen.getAllByRole("option")

  expect(options()).toHaveLength(3)
  expect(input).toHaveAttribute("aria-activedescendant", options()[0].id)

  await user.keyboard("{ArrowDown}")
  expect(input).toHaveAttribute("aria-activedescendant", options()[1].id)

  await user.keyboard("{ArrowUp}")
  expect(input).toHaveAttribute("aria-activedescendant", options()[0].id)

  await user.keyboard("{ArrowDown}{ArrowDown}")
  expect(input).toHaveAttribute("aria-activedescendant", options()[2].id)

  await user.keyboard("{Enter}")
  expect(onValueChange).toHaveBeenCalledWith("79")
  expect(screen.queryByRole("listbox")).not.toBeInTheDocument()
})

it("filters options as the user types and keeps activedescendant on the filtered list", async () => {
  const { onValueChange } = renderSelect()
  const user = userEvent.setup()

  await user.click(screen.getByRole("button", { name: "Chọn tỉnh/thành phố" }))
  await user.type(screen.getByRole("combobox"), "Đà N")

  const options = screen.getAllByRole("option")
  expect(options).toHaveLength(1)
  expect(options[0]).toHaveTextContent("Đà Nẵng")
  expect(screen.getByRole("combobox")).toHaveAttribute("aria-activedescendant", options[0].id)

  await user.keyboard("{Enter}")
  expect(onValueChange).toHaveBeenCalledWith("48")
})

it("closes with Escape and restores focus to the trigger", async () => {
  renderSelect()
  const user = userEvent.setup()

  const trigger = screen.getByRole("button", { name: "Chọn tỉnh/thành phố" })
  await user.click(trigger)
  expect(screen.getByRole("listbox")).toBeInTheDocument()

  await user.keyboard("{Escape}")

  expect(screen.queryByRole("listbox")).not.toBeInTheDocument()
  expect(trigger).toHaveAttribute("aria-expanded", "false")
  expect(document.activeElement).toBe(trigger)
})
