import { describe, expect, it, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { CustomFieldFilterMenu } from "@/components/customFields/CustomFieldFilterMenu"
import { customFieldHandlers } from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"

const diesel = { id: "opt-diesel", label: "Diesel" }
const petrol = { id: "opt-petrol", label: "Petrol" }
const fuel = customFieldHandlers.field({
  id: "cf-fuel",
  name: "Fuel",
  type: "select",
  options: [diesel, petrol],
})

describe("<CustomFieldFilterMenu />", () => {
  it("leaves out a picked option the field no longer has", async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    const { rerender } = render(
      <CustomFieldFilterMenu fields={[fuel]} conditions={[]} onChange={onChange} />
    )
    await user.click(screen.getByTestId("commodities-filter-custom-fields"))
    await pickRadixSelect(user, /^Field$/, { optionLabel: /^Fuel$/ })
    await user.click(screen.getByTestId("commodities-filter-cf-option-opt-petrol"))
    // The group's fields are read again, without Petrol, while the menu is
    // open: the picked option is no longer listed, so it cannot be sent.
    rerender(
      <CustomFieldFilterMenu
        fields={[{ ...fuel, options: [diesel] }]}
        conditions={[]}
        onChange={onChange}
      />
    )
    const add = screen.getByTestId("commodities-filter-cf-add")
    expect(add).toBeDisabled()
    await user.click(screen.getByTestId("commodities-filter-cf-option-opt-diesel"))
    await user.click(add)
    expect(onChange).toHaveBeenCalledWith(["cf-fuel:in:opt-diesel"])
  })
})
