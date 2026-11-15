# quote-bot

An Instagram account that runs itself. A Go program picks a post from a
hand-written queue, draws it as slides or a video, and publishes it through
Instagram's official API. GitHub Actions runs it three times a day, and there
is no server to look after.

## What it posts

Posts are picked at random from `posts.json`, never the same series twice in a
row and never the same post twice.

| Series | What it is | Format |
|---|---|---|
| Scene Study | One film scene and what makes it work | Carousel |
| Episode Study | The same for one episode of a series | Carousel |
| What If | A famous scene with one thing changed | Carousel |
| Did You Notice | Details hidden in well-known films and series | Carousel |
| Look Closer | A painting, whole and then in close-up | Carousel |
| Shelf Notes | A book or a poem | Carousel |
| Liner Notes | A record | Carousel |
| One Frame | One image and one fact about how it was made | Single image |
| One-Liner | A stand-up joke | Single card |
| Hot Take | An opinion to argue with | Single card |
| Random Thought | An observation about films, TV or music | Single card |

Any carousel can also go out as a **Reel**: a vertical video that shows each
slide long enough to read, with a progress bar.

## Schedule

All times are UTC and chosen so that each part of the world gets one post in
its evening.

| Time (UTC) | What | Evening for |
|---|---|---|
| 13:07 | Post | India |
| 17:37 | Reel | Europe |
| 01:07 | Post | The Americas |

Change the `cron` lines in `.github/workflows/post.yml` and `reel.yml` to move
them.

## How a run works

1. `quotebot render` picks a post, fetches an image for it, and draws the
   slides (or the video) into `out/`.
2. The workflow commits `out/` so the files have a public address.
3. `quotebot publish` hands those addresses to Instagram and waits for it to
   finish processing.
4. Only then is the post added to `posted.json`. If Instagram refuses, nothing
   is recorded and the next run tries again.

### Where images come from

Each kind of post tries its sources in order and uses the first that answers.
If none does, the post goes out as text only.

| Kind | Sources, in order |
|---|---|
| Film, series | TMDB stills, TMDB poster, Apple poster, Wikipedia |
| Painting | Art Institute of Chicago (public domain), Wikipedia |
| Record | Apple Music cover, Wikipedia |
| Book | Open Library cover, Wikipedia |

Film stills, posters and covers are copyrighted. Accounts like this one use
them for commentary, but a rights holder can ask Instagram to remove a post.
Paintings from the Art Institute of Chicago are public domain.

## Limits

- **No music.** Instagram's API cannot attach a song. Reels are silent. To post
  one with music, run **Post a Reel** with *publish* set to *no*, download the
  video from the run's **Artifacts**, and upload it from the app.
- **No Stories, no replies, no editing** of posts already published.

## Running it by hand

In the repository's **Actions** tab:

- **Post to Instagram**: posts now. Type a post's `id` to choose which.
- **Post a Reel**: posts a video, or only makes the file.
- **Checks**: runs the tests and validates `posts.json`. It runs by itself on
  every change.
- **Post (old Python bot)**: an emergency fallback. See below.

## Secrets

Set under **Settings → Secrets and variables → Actions**.

| Name | Needed | What it is |
|---|---|---|
| `IG_TOKEN` | Yes | Instagram access token from the Meta developer dashboard |
| `IG_USER_ID` | Yes, for tokens starting `EAA` | The Instagram account ID shown on that dashboard |
| `TMDB_KEY` | No | A free TMDB API key. Without it film posts use posters instead of stills |

## Adding posts

`CONTENT_GUIDE.md` describes the voice, the rules and the exact shape of each
series. A scheduled Claude task adds a batch every week by following it. To add
posts yourself, append entries to `posts.json` and run:

```bash
go run ./cmd/quotebot validate
```

The Checks workflow runs the same command on every push, so a mistake in
`posts.json` shows up as a failed check instead of a failed post.

## Working on the code

```bash
go test ./...
```

```bash
go run ./cmd/quotebot preview
```

```bash
go run ./cmd/quotebot status
```

`preview` draws every post into `preview/` without touching Instagram. Add
`-images` to fetch real pictures, and `-id some-post` to draw only one.

| Path | What is in it |
|---|---|
| `cmd/quotebot` | The command-line program and the end-to-end tests |
| `internal/content` | Queue types, validation, random selection, captions |
| `internal/images` | Image sources and their fallback order |
| `internal/render` | Slide drawing; fonts are embedded in the program |
| `internal/reel` | Video building with ffmpeg |
| `internal/instagram` | The publishing client, with retries |
| `posts.json` | The queue |
| `quotes.json` | Plain quote cards, used only if the queue ever runs dry |
| `posted.json` | What has been published. Written by the bot; do not edit |
| `config.json` | Handle, hashtags, closing lines |
| `out/` | Drawn files from the last two weeks |

Reels need `ffmpeg` on the machine. Everything else is pure Go.

## The old Python bot

The original Python version is kept in `legacy/` and has its own workflow,
**Post (old Python bot)**, that you can run by hand if the Go bot is failing.
It posts one carousel. It can be deleted once the Go bot has run cleanly for a
couple of weeks.

## If a run fails

Open **Actions**, click the failed run, and read the step with the red cross.

| Message | Cause |
|---|---|
| `is not publicly reachable` | The repository is private, or the push step failed |
| `Instagram API error ... code 190` | The token is wrong or has been revoked |
| `could not work out the Instagram account ID` | Add the `IG_USER_ID` secret |
| `images: ... failed` | Not a failure. That source was skipped and the next one tried |

## Credits

Fonts are Playfair Display and Inter, both under the SIL Open Font License
(see `internal/render/fonts/`). This product uses the TMDB API but is not
endorsed or certified by TMDB.
