# quote-bot

An Instagram account that runs itself. A Go program picks a post from a
hand-written queue, draws it as slides or a video, and publishes it through
Instagram's official API. GitHub Actions runs it twice a day, and there is no
server to look after.

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
| Deadpan | An original long joke, told flat, one beat per slide | Carousel |
| Roast | Insult comedy aimed at a fictional character | Carousel |
| Short Story | A complete absurd story in a few slides | Carousel |

Any carousel can also be made into a **Reel**: a short animated vertical video
in which the hook, each point and the closing line move in over the film still
or a plain backdrop, with progress bars along the top.

## Schedule

All times are UTC and chosen so that each part of the world gets one post in
its evening.

| Time (UTC) | What | Evening for |
|---|---|---|
| 13:07 | Post | India |
| 17:37 | Comedy | Europe |
| 01:07 | Post | The Americas |

Reels are not scheduled. **Make a Reel** builds one for you to post from the
app with a song of your choice.

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

- **No music.** Instagram's API cannot attach a song, so **Make a Reel** does
  not publish by default. Open the finished run: its summary has a link to the
  video and the caption to paste. Save the video on your phone, post it as a
  Reel from the app, and add a song there. The bot marks the post as used so it
  never comes up again.
- **No Stories, no replies, no editing** of posts already published.

## Running it by hand

In the repository's **Actions** tab:

- **Post to Instagram**: posts now. Type a post's `id` to choose which.
- **Post comedy**: posts from the humour series only (Deadpan, Roast, Short
  Story, One-Liner). You can choose which kind. It also runs daily at 17:37 UTC.
- **Make a Reel**: makes an animated video and its caption for you to post
  with music. Set *publish* to *yes* only if you want the bot to post it silent.
- **Checks**: runs the tests and validates `posts.json`. It runs by itself on
  every change.
- **Check setup**: tests the secrets, the connection to Instagram and TMDB, and
  the queue, and says in plain words what is wrong. Run this first when a post
  fails.
- **Collect stats**: saves likes and comments per post to `stats.json`. It runs
  by itself every Saturday.
- **Post (old Python bot)**: an emergency fallback. See below.

### Starting a run with a commit

Committing a post's `id` into `.github/run-post` (or `.github/run-reel`) starts
a run for that post, and the word `next` picks a random one. This exists so a
Claude session, which can push commits but cannot press buttons on GitHub, can
post on request.

### When a run fails

The bot saves the last lines of its output to `last-error.log` in the
repository, and removes the file after the next successful run. GitHub also
emails the repository owner about the failed run.

## Secrets

Set under **Settings → Secrets and variables → Actions**.

| Name | Needed | What it is |
|---|---|---|
| `IG_TOKEN` | Yes | Instagram access token from the Meta developer dashboard |
| `IG_USER_ID` | Yes, for tokens starting `EAA` | The Instagram account ID shown on that dashboard |
| `TMDB_KEY` | No | A free TMDB API key. Without it film posts use posters instead of stills |

## The feedback loop

Every Saturday **Collect stats** records how each published post did and
averages the numbers per series in `stats.json`. An hour and a half later the
weekly writing task reads that file and writes more of what people responded
to, while keeping the mix varied. Likes and comments are the only numbers the
current token can read; reach, saves and sends would need the
`instagram_manage_insights` permission added on the Meta dashboard.

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
| `internal/render` | Slide drawing and Reel animation; fonts are embedded in the program |
| `internal/reel` | Video encoding with ffmpeg |
| `internal/instagram` | The publishing client, with retries |
| `posts.json` | The queue |
| `quotes.json` | Plain quote cards, used only if the queue ever runs dry |
| `posted.json` | What has been published. Written by the bot; do not edit |
| `stats.json` | Likes and comments per post and per series. Written by the bot |
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
