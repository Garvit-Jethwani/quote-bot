# Content guide

How to write new posts for this account. Every post is one entry in
`posts.json`. The bot picks unposted entries at random and never repeats one.

## Voice

The account reads like one person with wide taste and strong opinions: films,
series, paintings, books, records, stand-up. Specific and a little dry. Never
generic praise, never a list of facts with no point of view.

- **Slides** are in normal sentence case. Each body slide makes one point,
  with a concrete detail (a number, a name, what is in the frame) and says why
  it matters. Under 55 words per body.
- **Captions** are lowercase, first person, short, like a text to a friend.
  One or two short paragraphs. A personal reaction, not a summary.
- **Hooks** (the big line on slide one) state something surprising or pose
  the question. Under 14 words.

## Hard rules

- No em dashes or en dashes anywhere, and no arrows. Use commas, full stops
  or colons.
- No music recommendations and no `pairs` field.
- No image credits or sources in captions (the bot handles captions' credit
  line itself).
- `tags`: three or four specific hashtags (the work, the maker, the scene).
  The bot adds broad ones and caps the total at five.
- Facts must be ones you are confident of. Hedge the rest ("reportedly",
  "the story goes") or leave it out. Quote lines only when sure of the wording.
- Nothing explicit, hateful or cruel. Dry and dark is fine.
- Never reuse an `id`, and do not cover a work that already has a post in the
  same series.

## Series and their shape

Every entry needs `id`, `kind`, `label`, `title`, `slides`, `caption`, `tags`.
Multi-slide posts also need `hook` and `year`.

| Series | `kind` | `label` | Extra fields | Slides |
|---|---|---|---|---|
| Scene Study | `film` | `SCENE STUDY` | `director` | 3 or 4 |
| What If | `film` or `series` | `WHAT IF` | `director`, or the series fields below | 3 |
| Episode Study | `series` | `EPISODE STUDY` | `creator`, `first_aired`, `episode: [season, number]` | 3 |
| One Frame | `film` | `ONE FRAME` | `director` | none (`"slides": []`) |
| Look Closer | `art` | `LOOK CLOSER` | `artist`, `image: {"aic": "Exact title"}`, `detail: [x, y]` | 3 or 4 |
| Shelf Notes | `book` | `SHELF NOTES` | `author` | 3 or 4 |
| Liner Notes | `music` | `LINER NOTES` | `artist` | 3 or 4 |
| One-Liner | `comedy` | `ONE-LINER` | `quote` (the joke); `title` is the comedian | none |

A slide is either `{"k": "SMALL LABEL", "h": "Heading", "b": "Body text"}` or
a quotation `{"q": "The line", "by": "Plain words saying where it is from"}`.

Notes:

- For `film` and `series`, `title` and `year` must match TMDB so the right
  stills are found. For a series, `year` is the episode's year and
  `first_aired` is the show's first year.
- For `art`, the painting must be in the Art Institute of Chicago's
  public-domain collection, and `image.aic` must be contained in its title
  there. `detail` is where the close-up centres, as fractions across and down.
- What If posts rewind a well-known scene, change one thing, and follow the
  consequences. The last slide says what the real version understood.
- Look at existing entries in `posts.json` for examples of each series.

## Checking a batch before committing

```bash
NO_IMAGES=1 python3 bot.py preview
```

It must finish without errors. Then delete the `preview/` folder, confirm
`posts.json` is valid JSON with unique ids, and search the new entries for
dashes and arrows. Do not change `posted.json` or anything in `out/`.
