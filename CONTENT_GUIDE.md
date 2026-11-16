# Content guide

How to write new posts for this account. Every post is one entry in
`posts.json`. The bot picks unposted entries at random and never repeats one.

## Voice

The account reads like one person with wide taste and strong opinions: arthouse
and mainstream films, series, paintings, books, indie records, stand-up, and
whatever else is on their mind. Specific, dry, sometimes rude. Never generic
praise, never a list of facts with no point of view. Variety is the point: a
Tarkovsky frame can sit next to a joke about Shrek.

- **Slides** are in normal sentence case. Each body slide makes one point with
  a concrete detail (a number, a name, what is in the frame) and says why it
  matters. Under 60 words per body.
- **Captions** are lowercase, first person, short, like a text to a friend.
  One or two short paragraphs. A personal reaction, not a summary.
- **Hooks** (the big line on slide one) state something surprising or pose the
  question. Under 16 words. The hook decides whether anyone swipes.
- **Swearing is fine** where it is funny or the original joke has it. Use it
  like seasoning. Nothing sexual, hateful, or cruel to real people.

## What works on Instagram

- Carousels do best with real substance: write **four** text slides when the
  material allows, not three. With the cover and the closing slide that makes
  six.
- The closing slide asks for a send, a save or a reply, which are the actions
  Instagram weighs most. The bot adds it from `config.json`. For posts built
  around a debate, set `"ask"` to a specific question instead.
- Search reads captions. The bot appends a plain credit line (title, maker,
  year), so the caption itself does not need to repeat it.
- Instagram allows five hashtags. Give three or four specific ones in `tags`
  (the work, the maker, the scene); the bot adds one or two broad ones.

## Hard rules

- No em dashes, en dashes or arrows anywhere. Use commas, full stops or colons.
- No music recommendations and no `pairs` field.
- No image credits or sources in captions.
- Facts must be ones you are confident of. Hedge the rest ("reportedly", "the
  story goes") or leave it out. Quote lines and jokes only when sure of the
  wording.
- Never reuse an `id`, and do not cover a work that already has a post in the
  same series.

## Series and their shape

Every entry needs `id`, `kind`, `label`, `title`, `slides`, `caption`, `tags`.
Multi-slide posts also need `hook`, `year` and a maker.

| Series | `kind` | `label` | Extra fields | Text slides |
|---|---|---|---|---|
| Scene Study | `film` | `SCENE STUDY` | `director` | 3 to 5 |
| What If | `film` or `series` | `WHAT IF` | maker; `ask` recommended | 3 or 4 |
| Episode Study | `series` | `EPISODE STUDY` | `creator`, `first_aired`, `episode` | 3 or 4 |
| Did You Notice | `film` or `series` | `DID YOU NOTICE` | maker | 3 to 5 |
| Look Closer | `art` | `LOOK CLOSER` | `artist`, `image`, `detail` | 3 to 5 |
| Shelf Notes | `book` | `SHELF NOTES` | `author` | 3 to 5 |
| Liner Notes | `music` | `LINER NOTES` | `artist` | 3 to 5 |
| One Frame | `film` or `series` | `ONE FRAME` | maker | none |
| One-Liner | `comedy` | `ONE-LINER` | `quote`; `title` is the comedian | none |
| Hot Take | `take` | `HOT TAKE` | `quote`; `title` is `Hot take` | none |
| Random Thought | `thought` | `RANDOM THOUGHT` | `quote`; `title` is `Random thought` | none |
| Mood | `mood`, `film`, `series` or `art` | `MOOD` | `quote`; see below | none |
| Deadpan | `bit` | `DEADPAN` | `hook`; `title` is a dry subtitle | 3 to 6 pages |
| Roast | `bit` | `ROAST` | `hook`; `title` is where the target is from | 3 to 6 pages |
| Short Story | `bit` | `SHORT STORY` | `hook`; `title` is `A very short story` | 3 to 6 pages |

A slide is either `{"k": "SMALL LABEL", "h": "Heading", "b": "Body text"}` or a
quotation `{"q": "The line", "by": "Plain words saying where it is from"}`.
The three humour series use plain pages instead: `{"t": "One beat of the joke
or story"}`, up to 48 words each. They need no `year` or maker.
Posts with no text slides use `"slides": []`.

Notes on each:

- **Film and series**: `title` and `year` must match TMDB so the right stills
  are found. For a series, `creator` is the maker, `year` is the episode's
  year, `first_aired` is the show's first year, and `episode` is
  `[season, number]`.
- **What If**: rewind a well-known scene, change one thing, follow the
  consequences. The last slide says what the real version understood. Mix
  arthouse and blockbuster.
- **Did You Notice**: three to five details most viewers miss, ending on why
  the film bothered.
- **Look Closer**: the painting must be in the Art Institute of Chicago's
  public-domain collection, with `image: {"aic": "Exact title"}` contained in
  its title there. `detail` is where the close-up centres, as fractions across
  and down, such as `[0.7, 0.4]`.
- **One-Liner**: a real joke by a real comedian, worded as they said it.
- **Hot Take** and **Random Thought**: the account's own voice, one or two
  sentences, written to be argued with or sent to a friend. The caption backs
  it up or makes it worse.

- **Mood**: one image in the manner of a meme: a single relatable line and a
  picture, nothing else. Lowercase, first person, under 26 words. About half
  should be funny (small daily failures, overthinking, music taste, work) and
  half quietly sincere (boredom, routine, being alone and fine with it). No
  advice-poster wisdom. Three forms:
  - `kind: "mood"` with a `doodle` starting `guy-` (`guy-bed`, `guy-cloud`,
    `guy-desk`, `guy-floor`, `guy-fridge`, `guy-headphones`, `guy-party`):
    the line over a marker cartoon. `title` is `Mood`.
  - `kind: "film"` (or `series`) with `title`, `year` and maker: the line is
    set over a still from it. Pick films whose look carries the feeling.
  - `kind: "art"` with `artist`, `year` and `image.aic`: the line above a
    public-domain painting, read as if it showed modern life.

- **Deadpan**: an original long joke told flat, in the first person, one beat
  per page. The setup is ordinary, the delivery is patient, and the last page
  undercuts everything before it. Think of a man telling a story at a bar who
  does not know it is funny. Blunt, a little dark, swearing allowed.
- **Roast**: insult comedy aimed at a fictional character, one burn per page,
  affectionate underneath. Never roast a real person. Every burn must be based
  on something the character actually does on screen.
- **Short Story**: a complete absurd story in three to six pages. One strange
  premise, played completely straight, with an ending that lands. The hook is
  the first sentence of the story.
- **Drawings**: give each of these three a `"doodle"`, a small hand-drawn
  picture shown on its slides and animated in its Reel. Pick the object the
  joke turns on. Available: `bat`, `bone`, `book`, `bulb`, `clapper`, `clock`,
  `dumbbell`, `flame`, `fridge`, `ghost`, `lift`, `martini`, `mic`, `mug`,
  `parrot`, `phone`, `plate`, `ship`, `van`, `wifi`, `wizard`. If none fits,
  leave the field out rather than forcing one. One-Liner, Hot Take and Random
  Thought cards get a drawing by themselves.
- These three post through their own workflow (**Post comedy**) and are kept
  out of the main rotation by `separate_series` in `config.json`.

Look at existing entries in `posts.json` for examples of each series.

## Checking a batch before committing

```bash
go run ./cmd/quotebot validate
```

It must report the queue as valid. It checks ids, required fields, slide
counts, word limits, tag counts and the no-dash rule. Then run the tests:

```bash
go test ./...
```

Do not change `posted.json` or anything in `out/`.
