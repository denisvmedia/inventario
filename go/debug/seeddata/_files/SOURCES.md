# Seed photo sources

The six committed `photo-*.jpg` files in this directory come from
[Pexels](https://www.pexels.com), under the
[Pexels License](https://www.pexels.com/license/), which permits
commercial use without attribution. Crediting is encouraged anyway —
the source IDs below let future maintainers refresh or replace any
single photo without re-deriving the slot semantics.

| File | Pexels photo ID | URL (size: 400x300 cropped) |
| --- | --- | --- |
| `photo-livingroom.jpg` | 1648776 | https://www.pexels.com/photo/1648776/ |
| `photo-kitchen.jpg`    | 2089698 | https://www.pexels.com/photo/2089698/ |
| `photo-bedroom.jpg`    | 271624  | https://www.pexels.com/photo/271624/ |
| `photo-work.jpg`       | 1957477 | https://www.pexels.com/photo/1957477/ |
| `photo-outdoor.jpg`    | 100582  | https://www.pexels.com/photo/100582/ |
| `photo-storage.jpg`    | 5025639 | https://www.pexels.com/photo/5025639/ |

## Refreshing

Pick a new photo on Pexels, grab its numeric ID from the URL, then:

```sh
ID=271624
curl -sSL \
  "https://images.pexels.com/photos/$ID/pexels-photo-$ID.jpeg?auto=compress&cs=tinysrgb&w=400&h=300&fit=crop" \
  -o photo-bedroom.jpg
```

Aim for ~10–35 KB per file at the 400x300 thumbnail size — the seed
uses these as cover photos on every commodity, so total embed size
stays well under 200 KB across the six slots.

## PDF fixtures

`invoice.pdf` and `manual.pdf` are hand-rolled by `gen_fixtures.go` (a
minimal 1-page PDF apiece). They are NOT Pexels-sourced; they have no
licensing baggage. Run `go run gen_fixtures.go` to regenerate them.
The same script can also emit synthetic colored-swatch JPGs as a
licensing-clean fallback if the Pexels photos ever need to be removed.
