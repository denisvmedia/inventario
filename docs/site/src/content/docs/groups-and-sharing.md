---
title: Groups and sharing
description: Keep separate inventories in groups, set each one's currency, and invite people with the right role.
---

Inventario organises everything you track into **groups**. A group is a complete, self-contained inventory — its own [locations and areas](../locations-and-areas/), [items](../items/), [files](../files-and-photos/), [tags](../tags/), [sellers](../sellers/), [item types](../item-types/), and [backups](../backup-and-restore/). You can belong to several groups at once (your home, a holiday flat, a small business), switch between them whenever you like, and invite other people into any group with a role that decides what they can do.

![The Members page showing the member list, role permissions card, and member stats](../../assets/screenshots/members.png)

## What a group is

Think of each group as a separate inventory that doesn't mix with your others. Items, locations, files, and tags all live inside one group — switching groups swaps the whole view in the app. The people you invite into a group only ever see that group's data, never your other groups.

The sidebar always shows which group you're currently working in. If you don't belong to any group yet, for example as the first administrator of a new server, the app opens a welcome page that asks you to create one.

## Create a group

You can create as many groups as you need.

1. Open the group selector at the top of the sidebar (it shows the current group's name and member count).
2. Choose **Create new group**.
3. On the **Create a new group** page, fill in:
   - **Group name** — for example "Main residence".
   - **Icon** — optional; pick something to make the group easier to recognise in the sidebar.
   - **Group currency** — the currency this inventory's prices are tracked in (see [The group currency](#the-group-currency)).
   - **What is the group for?** — **Household items** (the default) or **Collection**. A **Collection** group starts with the [Collection set](#add-the-collection-set) of item types and custom fields and with the **Warranties** section [hidden](#hide-sections-the-group-doesnt-use). You can change both later in the group settings.
4. Click **Create group**. The new group opens straight away and becomes your active group. If it is your first group, the [product tour](../getting-started/) starts and shows you around the sidebar.

If the group is created but the Collection set could not be added, a message says so. The group is there; open **Group settings → Sections** and click **Add the set**, or **Add what is missing** if part of it is there, to finish it.

## The group currency

Every group has one **group currency** — the currency all of its item prices and totals are shown in. You choose it when you create the group, using a 3-letter ISO code (for example **USD**, **EUR**, or **CZK**).

:::caution[Set once at creation]
The group currency is fixed when you create the group. To change it later you'd use the currency-migration tool in **Group settings → Info**, which reprices every item in the group at an exchange rate you provide. That tool is only available if your server operator has enabled it; if it's switched off you'll see a note saying so.
:::

## Switch the active group

The active group decides what the whole app shows you.

1. Open the group selector at the top of the sidebar.
2. Pick the group you want from the list. A check mark shows which one is currently active.

The app switches to that group immediately.

:::tip
You can keep two browser tabs open on two different groups — they stay independent of each other. The same dropdown also has shortcuts to **Group settings** (for the current group) and **Create new group**.
:::

## Hide sections the group doesn't use

Not every inventory needs every part of Inventario: a group for a book collection has no warranties to watch, for example. An administrator or owner can hide these sections for everyone in the group:

| Section | What hiding it removes |
| --- | --- |
| **Warranties** | The **Warranties** page, the warranty cards on the Dashboard, the warranty counts on area and location pages, the **Warranty** step of the item form, and warranty reminder emails. |
| **Maintenance and repairs** | The **Maintenance** and **In service** pages, adding a schedule or sending an item for service, and maintenance reminder emails. |
| **Loans** | The **Lent out** page, lending an item out, and loan reminder emails. |
| **Upcoming** | The **Upcoming** page. |

To hide a section or show it again:

1. Open the group selector and choose **Group settings**, then open the **Sections** section.
2. Switch a section off to hide it, or on to show it. The change applies at once, for every member.

Members with the **User** or **Viewer** role see the same switches but can't change them.

Hiding a section keeps its data. An item that already has a warranty date, a loan, or a maintenance schedule still shows it on its page, and the item form keeps the **Warranty** step for an item that has a warranty. You can still mark a loan returned or a task done; you can't start a new loan, schedule, or service visit until the section shows again. While a section is hidden, its reminder emails stop and the [weekly digest](../settings-and-account/#weekly-digest) leaves its lines out. When you show it again, reminders pick up from the dates as they are then, as described in [Reminder emails](../warranties-loans-maintenance/#reminder-emails).

A link or a bookmark to a hidden page opens a notice that the section is hidden, with a link to the group settings.

## Add the Collection set

The **Collection set** fits a group that keeps a collection. Adding it creates these [item types](../item-types/):

**Art**, **Books**, **Coins**, **Stamps**, **Records**, **Watches**, and **Cards**

and these [custom fields](../custom-fields/), each offered on every one of those types:

| Field | Type |
| --- | --- |
| **Condition** | Select: Mint, Very good, Good, Fair, Poor |
| **Catalog reference** | Text, for a Krause, Scott, Discogs, or ISBN number and the like |
| **Edition or variant** | Text |
| **Signed** | Yes/no |
| **Provenance** | Long text |
| **Certificate of authenticity** | Yes/no |
| **Appraised value** | Money |
| **Appraisal date** | Date |

The item's own **Serial number** stays as it is. The types and fields are created with English names; rename them, change their icons, or add options like any type or field you made yourself.

To add the set:

1. Open the group selector and choose **Group settings**, then open the **Sections** section.
2. On the **Collection set** card, click **Add the set**.

If the group already has a type or field with one of these names (upper and lower case don't matter), the set uses it instead of creating a second one: such a type gets the set's fields, and such a field is offered on the set's types. Adding the set again creates only what is missing, so if adding stopped partway, for example because the group reached its limit of [item types](../item-types/#add-a-type) or custom fields, the card shows **Partly added** and **Add what is missing** finishes it.

To remove the set, click **Remove the set** on the same card. A confirmation lists what will be deleted and what stays, with the reason:

- a type or field the group had before the set, which the set never deletes;
- a type that items have, or a field that items have a value of, counting items in the [Trash](../trash/);
- the group's last item type, as a group always keeps one.

Removing deletes the rest and leaves every item as it is. You can remove the set again later: it then deletes what has stopped being used since.

Only an **Owner** or **Administrator** can add or remove the set. Other members see the card without the buttons. Adding and removing the set are recorded in the audit log.

## Invite people by email

You invite people from the **Members** page. You need to be an **Owner** or **Administrator** of the group to send invites.

1. Open **Members** in the sidebar.
2. Click **Invite**.
3. In the **Invite to …** dialog, enter the person's **Email address**.
4. Choose a **Role** — **Viewer**, **User**, or **Administrator** (you can't invite someone straight in as Owner; see [Members and roles](#members-and-roles)).
5. Click **Send invite**. They'll receive an invite link by email.

### Copy-paste invite links

If you'd rather hand someone a link yourself instead of emailing it, choose **Or create a copy-paste invite link instead** in the invite dialog. Inventario generates a link you can copy and share however you like — by message, chat, or in person. Anyone who opens a valid link can use it to join the group with the role you picked.

## Members and roles

Each member of a group has one **role** that controls what they can do. The **Members** page shows everyone in the group with their role, and a **Role permissions** card explains each role. Members who have set a [profile photo](../settings-and-account/#profile-photo) appear with it; everyone else appears with their initials.

| Role | What they can do |
| --- | --- |
| **Owner** | Full access including deleting the group and all data. |
| **Administrator** | Can manage members, group info, locations and areas. |
| **User** | Can add and remove items (not locations or areas). |
| **Viewer** | Can view all items, but cannot make any changes. |

A group can have more than one owner. The **Members** page also shows quick stats at the top: total **Members**, **Admins & Owners**, and **Pending invites**.

Every member, viewers included, can keep personal [saved views](../items/#saved-views) of the item list. Sharing a view with the group takes the **User** role or higher, and administrators and owners can change or delete any shared view. A shared view shares the filter only: each member sees the items their own access allows. Sharing does not put the view in anyone's sidebar or on their Dashboard; each member decides that for themselves.

:::note
If you're a User or Viewer, you can see the member list but you won't see the invite, role-change, or remove controls — those are limited to admins and owners.
:::

## Change a member's role

You need to be an **Owner** or **Administrator**.

1. Open **Members** in the sidebar.
2. Find the member and open their actions menu (the **⋯** button on their row).
3. Under **Change role**, pick the new role.

A few rules apply:

- Only **owners** can promote someone to **Owner** or change another owner's role. Administrators can manage everyone except owners.
- You can't change your own role from this menu.
- You can't demote the **last owner**. Promote someone else to owner first, or [delete the group](#delete-a-group) instead. The app blocks this for you and explains why.

## Manage pending invites

Invites that haven't been accepted yet appear in the **Invite links** section of the **Members** page, each marked with a **Pending** badge. Owners and administrators can act on them via the **⋯** menu on each invite row:

- **Resend invite** — send the invite email again (available for email invites).
- **Copy** — copy the invite link to your clipboard so you can share it another way.
- **Revoke** — cancel the invite. The link stops working and the person can no longer use it to join.

Invites that have already been used can't be revoked.

## Leave a group

If you no longer want to be part of a group, you can leave it.

1. Open the group selector and choose **Group settings**, then open the **Members** section.
2. In the **Leave group** panel, click **Leave group**.

You'll lose access to all of that group's data, but you can be re-invited later.

:::caution[Last owner can't leave]
You can't leave a group if you're its **last owner** — that would strand the group with no one able to manage it. Promote another member to owner first, or delete the group. If you're the only member at all, delete the group instead of leaving.
:::

## Remove a member

Owners and administrators can remove other people from a group.

1. Open **Members** in the sidebar.
2. Open the **⋯** menu on the member's row and choose **Remove**.
3. Confirm in the dialog.

The person loses access to the group's data immediately, and you can re-invite them later. You can't remove the **last owner** (promote someone else first), and the menu doesn't offer a remove action on your own row — use [Leave a group](#leave-a-group) for that.

## Delete a group

Deleting a group is permanent and removes **all** of its locations, items, files, and exports. Only an **Owner** can do it.

![Group settings page with the management and danger zone sections](../../assets/screenshots/group-settings.png)

1. Open the group selector and choose **Group settings**, then open the **Management** section.
2. Under **Danger zone**, click **Delete group**.
3. In the confirmation dialog, type the **group name** and your **current password**, then click **Delete permanently**.

The group name guards against an accidental click; your password protects against someone using a hijacked session.

:::caution[No undo]
There's no way to recover a deleted group. If you might want the data later, create a [backup](../backup-and-restore/) first.
:::

## Related guides

- [Locations and areas](../locations-and-areas/) — organise where things live inside a group.
- [Settings and account](../settings-and-account/) — your personal preferences and profile.
- [Backup and restore](../backup-and-restore/) — export a group's data before big changes.
