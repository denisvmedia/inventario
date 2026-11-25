# Item query contract

One representation of an item filter is shared by every surface that lists, searches or counts items. The same query string selects the same items, in the same tenant and group scope, on each of them:

- the items list, `GET /g/{slug}/commodities`, and its `meta.commodities` total;
- the item search, `GET /g/{slug}/search?type=commodities`, and its `meta.total`;
- the tag counts, `GET /g/{slug}/commodities/tag-counts`, behind the items list's Tags menu;
- saved views, `GET /g/{slug}/saved-views` and their counts, `GET /g/{slug}/saved-views/counts`;
- every frontend link that opens a filtered items list.

Features that select items (header search, bulk "select all N matching", the inbox) build on this contract rather than adding parameters of their own. A new filter is added here first, on both sides, and then used.

## Where it lives

| Part | Go | TypeScript |
| ---- | -- | ---------- |
| Parameters, parsing, validation, encoding | `go/services/commodityquery/query.go` (`Query`, `Parse`, `Query.Values`) | `frontend/src/features/commodities/query.ts` (`CommodityQuery`, `parseCommodityQuery`, `writeCommodityQuery`, `listOptionsToParams`) |
| Running a query, totals, match reasons, tag counts | `go/services/commodityquery/run.go` (`Resolve`, `List`, `Explain`, `TagCounts`) | `useCommodities`, `useCommodityTagCounts` in `features/commodities/hooks.ts` |
| What a search term and a tag filter match, how tags are counted | `go/registry/commodity_search.go` (`SplitSearchTerms`, `MatchCommoditySearch`, `CommodityTagFilter`, `CountCommodityTags`) | — |
| Backends | `go/registry/postgres/commodities.go` (SQL), `go/registry/memory/commodities.go` (Go) | — |
| Behavior both backends must share | `go/registry/registrytest/commodity_search.go` (run by the memory and postgres test packages) | `features/commodities/__tests__/query.test.ts` |

Handlers do not read filter parameters themselves. They decode the raw query string with `commodityquery.DecodeQuery`, which refuses a pair that `url.URL.Query` would drop (a malformed `%` escape, a `;` separator), and call `commodityquery.Parse` with the names they handle on their own (pagination on the list; `type`, `limit`, `offset` on `/search`; none on `/commodities/tag-counts`, which refuses `page`) and pass the result to `commodityquery.List` or `commodityquery.TagCounts`.

## Parameters

API names, as `GET /commodities` and `GET /search?type=commodities` read them:

| Parameter | Value | Meaning |
| --------- | ----- | ------- |
| `q` | text, at most 200 characters and 10 terms | Free-text search, see below. |
| `type` | repeatable | Item type is one of the values. Any value is accepted: types become per group (#2802), and an unused value matches nothing. Not available on `/search`, where `type` names the entity type. |
| `status` | repeatable, a known status | Item status is one of the values. |
| `area_id` | area id | Item is in that area. |
| `unassigned` | `true`/`false` | Item has no area. Ignored when `area_id` is set. |
| `include_inactive` | `true`/`false`, default `true` | `false` hides drafts and, unless `status` is set, items that are not in use. |
| `warranty_status` | repeatable: `active`, `expiring`, `expired`, `none` | Computed warranty status is one of the values. |
| `warranty_expires_before` | `YYYY-MM-DD` | Warranty ends before that day. |
| `lent_out` | `true`/`false` | Item is (or is not) on an open loan. |
| `tags` | slugs, comma-separated or repeated, at most 50 | Tags the item must carry, see `tag_mode`. |
| `tag_mode` | `all` (default) or `any` | `all`: every tag in `tags`; `any`: at least one. |
| `tags_exclude` | slugs, comma-separated or repeated, at most 50 | Item carries none of these tags. |
| `no_tags` | `true`/`false` | Item has no tags. |
| `sort` | field, `-` prefix for descending | Order. Not a filter: an unknown field falls back to `name`. An unknown date or price (NULL) comes after every known one in both directions, and a known zero sorts as a value; `registrytest.RunCommoditySearchContract` holds both backends to it. |

All conditions are combined with AND. Within a repeatable parameter the values are combined with OR, except `tags` with `tag_mode=all`.

Boolean values are `true`/`false` or `1`/`0`, in any case. Tag values are normalized like a tag slug on write, so `tags=Fragile` finds the tag `fragile`.

## Free-text search

`q` is split into terms at white space; double quotes keep a phrase together as one term (`"power drill" red` is two terms). Terms are compared without regard to case, and a repeated term counts once.

Every term must match. A term matches an item when one of these holds:

- it is a substring of one of the searched fields: name, short name, serial number, extra serial numbers, part numbers, tag slugs, notes (`comments`), warranty notes, status note;
- it is a substring of the label of an item tag the item carries (the labels are looked up per term in the item tag namespace before the query runs; file tags are never consulted);
- it is the item's asset ID (`A-0042`, `a-42`, `A42`, `#42` and `42` all name asset number 42, see `models.ParseAssetID`).

`%` and `_` are ordinary characters, as in a plain substring search.

In PostgreSQL the searched fields are joined into the generated column `commodities.search_text` (by the `commodity_search_text` SQL function), which carries a trigram GIN index, so a term is one `ILIKE` on an indexed column. The memory registry evaluates `MatchCommoditySearch` in Go. `registry.CommoditySearchFields` must list the same fields as the generated column; a unit test compares the two.

Not part of the contract yet: `field:value` syntax (`serial:ABC123`, `tag:fragile`) and the text of attached PDFs. Until they are added, a colon is an ordinary character in a term.

## Why an item matched

With a non-empty `q`, the response carries `meta.matches`, keyed by item id:

```json
"matches": {
  "6c1…": [
    { "field": "serial_number", "value": "SN-1234" },
    { "field": "tags", "value": "power-tools" }
  ]
}
```

`field` is one of `asset_id`, `name`, `short_name`, `serial_number`, `extra_serial_numbers`, `part_numbers`, `tags`, `comments`, `warranty_notes`, `status_note`, in that order. A list field reports the matching element; a tag reports its slug; a note reports an excerpt of up to 40 characters on each side of the first matching term, with an ellipsis where text was cut. The frontend shows the first match unless the name matched (`primarySearchMatch`), since the name is already the result's title.

## Tag filter

`tags`, `tag_mode`, `tags_exclude` and `no_tags` compare slugs. An item with `NULL` tags is treated as an item with no tags. The filter refuses combinations that cannot select anything:

- `no_tags=true` together with `tags` or `tags_exclude`;
- one tag both in `tags` and in `tags_exclude`.

### Tag counts

`GET /commodities/tag-counts` takes the same parameters as the list, without pagination, and answers:

```json
{ "data": { "tags": { "garage": 12, "fragile": 3 }, "untagged": 40 } }
```

The counts are for the query with its tag filter set aside: `tags[slug]` is the number of items the rest of the query selects that carry the tag, and `untagged` the number with no tags. So the counts do not change while the user picks tags, and a tag's count is the total the list shows when that tag alone is selected. A tag no selected item carries is absent from `tags`. The search, the other filters (`include_inactive` among them) and the group scope apply as on the list. `CountCommodityTags` is the reference: the memory registry calls it, and the shared registry tests check both backends against it applied to the listed items.

## Errors

A parameter the endpoint does not know, a value it cannot read or decode, a contradiction or a limit exceeded is answered with 400 and never ignored, because a list that silently drops a condition looks like a list that applied it:

```json
{
  "errors": [{
    "status": "Bad Request",
    "code": "query.invalid_parameter",
    "meta": { "parameter": "status", "reason": "unknown status" }
  }]
}
```

Next to these members, `error` carries the message as for any API error. The items list page shows this as an alert that names the parameter, with a Clear filters button.

## Saved views

A saved view stores a query string of this contract, in the API names, without paging. `commodityquery.ParseSaved` reads it: `Parse` limited to `SavedParams`, the filters and the sort the items list shows and edits. It refuses `warranty_expires_before`, an absolute date that a view meant to stay current must not keep (`warranty_status` is computed on every run instead), and `lent_out=false`, which the list has no control for. A view opens as the items list, so a condition the list cannot show would either be dropped on the way, which broadens the view, or stay invisible to the person looking at it.

The service validates the query on every write and stores `Query.Encode()`, the canonical form (sorted keys, and sorted values in the set-like lists: types, statuses, warranty statuses, included and excluded tags), with a query version (`models.SavedViewQueryVersion`). A stored query that no longer parses, or carries another version, is reported per view as `query_error` with the parameter and the reason; such a view has no count and is never run, so a contract change cannot silently broaden or change a view. The counts endpoint counts the views the requesting member placed in their sidebar or on their dashboard with `commodityquery.CountEach`: each query goes through `Resolve` and the WHERE clause of the items list (`CommodityRegistry.CountEach`), in one transaction and without reading rows, so the sidebar badge, the dashboard card and the opened list agree.

Placement is per member: `saved_view_placements` holds, per view and member, `show_in_sidebar` and `show_on_dashboard`. Any member who sees a view may place it; sharing a view places it for nobody else, and unsharing drops the placements of everyone but the creator.

Renaming an item tag's slug rewrites every view of the group that names it (`Query.RenameTag`). A deleted tag, area or location stays in the query: the view still applies the condition and shows what still matches, and the frontend names what no longer exists.

On the frontend, `commodityQueryToApiQuery` writes the stored form (sort and `include_inactive` always written) and `commodityQueryFromApiQuery` reads it back, an absent `include_inactive` being the server default. `sameCommodityQuery` compares two queries in that form, which is how the items list tells that an open view was changed.

## Frontend: page URL and API

The items list page keeps its query in the page URL under names that predate the contract, so saved links keep working. `writeCommodityQuery` replaces only these names and leaves every other parameter (page, view) as it is; `parseCommodityQuery` reads them back. Defaults are left out of the URL.

| Page URL | API |
| -------- | --- |
| `q` | `q` |
| `type`, `status` (repeated) | `type`, `status` |
| `area` | `area_id` |
| `unassigned=1` | `unassigned=true` |
| `inactive=1` | `include_inactive=true`; absent sends `include_inactive=false` |
| `warranty` (repeated) | `warranty_status` |
| `lent_out=1` | `lent_out=true` |
| `tags`, `tags_exclude` (comma-separated) | same |
| `tag_mode=any` | same, sent only with `tags`; the page URL keeps it without tags so the mode picked before the first tag survives |
| `no_tags=1` | `no_tags=true` |
| `sort` | `sort` |

`listOptionsToParams` is the only place that writes API parameters for the list. It sorts set-like lists, so two queries that select the same items produce the same string, which the TanStack Query key relies on. Links to a filtered list are built with `commodityListHref(slug, partialQuery)`.

Filter values are kept as written when the page URL is parsed, so a value the server does not support reaches it and comes back as a 400 the page can explain. The page-URL booleans (`unassigned`, `inactive`, `lent_out`, `no_tags`) and `tag_mode` are read into a boolean or a mode, but a value that is not one the server reads (`1`/`true`, `0`/`false` in any case, absent; `all`/`any`) is kept in `CommodityQuery.unreadable`, written back to the URL and sent to the server as written. The sort is the exception, as on the server. An edit in the Tags menu goes through `withTagFilter`, which drops an unreadable `tag_mode` or `no_tags`; the menu counts them as active conditions so its Clear button is offered for them.

The item page walks the list it was opened from (`?from=`) through `features/commodities/listContext.ts`, which parses that list URL with `parseCommodityQuery` too, so the page and the walk ask the same question under the same query key.

The search page links "see all" to the list with `inactive=1`, because `/search` includes drafts and inactive items and the list it opens has to show the same items. For the same reason the links that open the list for one tag (`commodityTagHref`, used on the item page and the Tags page) and the item page's area and type links carry `inactive=1`: the tag's usage count includes inactive items, and the item the link sits on may be inactive itself. A tag pill on an item card in the list is different: it adds the tag to the list's current query with `withIncludedTag`, keeping every other condition.
