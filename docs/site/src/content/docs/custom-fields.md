---
title: Custom fields
description: Add your group's own fields to items, such as a license plate, a lens mount or a contract number, and fill them in on each item.
---

Items come with a fixed set of fields: name, brand, purchase date, serial
number, and so on. **Custom fields** add the ones your group needs on top of
those, such as a car's license plate, a camera's lens mount, or the number of a
warranty contract. Each item fills in only the custom fields that apply to it.

Custom fields belong to a group, like [item types](../item-types/) and
[tags](../tags/). Every member of the group sees the same fields, and members
who may edit items may add, rename, and delete fields (see
[Groups & sharing](../groups-and-sharing/)). A field stays with the group when
the member who added it leaves or deletes their account.

## Open the Custom fields page

Click **Custom fields** in the **Group** section of the sidebar, or search for
"Custom fields" in the search window at the top of every page.

The page lists the group's fields by name. Each row shows the field's name, its
type, and the number of items that have a value for it. Items in the
[Trash](../trash/) are counted on their own, as "N in Trash". A **Select**
field also shows how many options it has.

Type in the search box to keep the fields whose name contains the text. Upper
and lower case don't matter.

## Add a field

1. Click **New field**.
2. Enter a **Name**, up to 60 characters. Each name is used once in the group:
   "plate" and "Plate" count as the same name.
3. Pick a **Type**. The type cannot be changed later.
4. For a **Select** field, add its options: click **Add option** for each one
   and type its label. A select field needs at least one option and keeps up to
   100, each with a label of up to 100 characters and used once in the field.
5. Click **Create field**.

A group keeps up to 200 custom fields. When it has that many, delete a field
before you add another.

The types and the values they take:

| Type | Value |
| --- | --- |
| **Text** | Up to 200 characters. |
| **Long text** | Up to 10,000 characters, on several lines. |
| **Number** | Up to 15 digits before the decimal point and 10 after it. |
| **Money** | An amount with its currency, up to 13 digits before the decimal point and 2 after it. |
| **Date** | A calendar date. |
| **Yes/no** | Yes or no. |
| **Select** | One of the field's options. |
| **Web address** | A link starting with `http://` or `https://`, up to 2,048 characters. |
| **Link to an item** | Another item of the group. |

## Rename a field or change its options

Point at a row and click the pencil. Items keep their values when you rename a
field or an option, so they show the new name at once.

On a select field you can rename options, add options, and remove them. Each
option shows how many items have it. Removing an option clears it from those
items when you save; the dialog says how many items that is before you click
**Save changes**.

## Delete a field

Point at a row and click the bin.

- When no item has a value for the field, confirm and the field is removed.
- When items have a value, the dialog says how many, Trash included. Tick
  **Delete the values too** and click **Delete field**: the field and every one
  of its values are deleted. This cannot be undone.

If another member gives an item a value for the field while the dialog is
open, the dialog shows the new count and asks you to confirm again.

## Fill in custom fields on an item

Open an item and click **Edit**. When the group has custom fields, the form has
a **Custom fields** section after **Notes & Tags**:

- The fields the item has a value for are listed with their values.
- **Add field** lists the group's other fields. Pick one to add it to the item.
- The **×** next to a field clears its value when you save.

The step-by-step form that **Add item** and **All fields** open has the same
section on its **Extras** step.

A field that is not on an item has no value, which is not the same as an empty
one: a number field can hold 0 and a yes/no field can hold "No". Clearing a
text field's text removes its value.

Each value is checked when you save, and a value the field does not take is
shown under the field.

### Link to another item

A **Link to an item** field names another item of the same group. Pick it from the
list, or type to search the group's items. An item cannot link to itself, and a
new link must name an item that is not in the Trash.

When the linked item is moved to the Trash later, the link stays and the item
page marks it "in Trash". When the linked item is deleted for good, the value
is removed.

## See custom fields on the item page

The item's **Details** tab lists the custom fields the item has a value for,
in the group's order, after the item's other fields. A link to another item
opens that item's page; a linked item in the Trash is shown by name with an
"in Trash" mark.

## Custom fields read by AI

When [Fill with AI](../files-and-photos/#scan-an-item-from-a-photo-or-pdf-with-ai)
reads a photo or a document, it can also fill in up to 25 of your group's
custom fields. **Money** and **Link to an item** fields are not filled in. The
values it read appear in the review list under the field's name, and only the
ones you keep ticked are filled in.

## Custom fields in reports and backups

The single-item [insurance report](../reports/) lists the custom fields the
item has a value for under **Item Details**.

A backup carries the group's custom fields and each item's values, and a
restore matches them to the fields of the group it restores into (see
[Backup & restore](../backup-and-restore/#custom-fields)).
