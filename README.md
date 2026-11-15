# Carousel bot for Instagram: films, books, records

Posts two multi-slide carousels a day to your Instagram account. It runs for
free on GitHub Actions, so there is no server to look after.

Each post is one of four series:

- **Scene Study**: a film, with real stills, and what makes one scene work.
- **Look Closer**: a painting, shown whole and then in close-up.
- **Shelf Notes**: a book or a poem.
- **Liner Notes**: a record.

Every post ends with a pairing slide ("play this with it", "read this after")
and has a hand-written caption.

## Two things to know first

**Music.** Instagram's API cannot attach a song from Instagram's music library
to a photo or carousel post. Only the app can. So each post names its song on
the last slide and in the caption instead.

**Film stills.** Stills come from TMDB. They are copyrighted by the studios.
Film accounts use them widely for commentary, and the bot credits the film and
director in every caption, but a rights holder can still ask Instagram to
remove a post. If you would rather not take that risk, skip the TMDB key and
film posts go out as text-only carousels.

**Where images come from.** Each post tries several sources in turn and uses
the first one that answers. If none does, the post goes out text-only.

| Series | Tried in this order |
|---|---|
| Scene Study | TMDB stills → TMDB poster → Apple poster → Wikipedia |
| Look Closer | Art Institute of Chicago (public domain) → Wikipedia |
| Liner Notes | Apple Music album cover → Wikipedia |
| Shelf Notes | Open Library book cover → Wikipedia |

Only TMDB needs a key. Without it, film posts use a poster instead of stills.
Posters and covers are copyrighted like stills are; paintings from the Art
Institute of Chicago are not. To post with no images at all, add a repository
variable or edit the workflow to set `NO_IMAGES: 1`.

## Setup (about 30 minutes, one time)

### 1. Instagram account

1. Create the account you want the bot to post to.
2. In the Instagram app: **Settings → Account type and tools → Switch to
   professional account**. Pick **Creator**. It is free.

### 2. Instagram access token

Meta renames these screens from time to time, so labels may differ slightly.

1. Go to <https://developers.facebook.com>, log in, click **My Apps → Create App**.
2. Choose the Instagram use case (**Manage messaging & content on Instagram**).
3. In the app dashboard open **Instagram → API setup with Instagram login**.
4. Under **Generate access tokens**, click **Add account** and log in with the
   Instagram account from step 1.
5. Click **Generate token** and copy it. This is `IG_TOKEN`. Treat it like a
   password.

### 3. TMDB key (for film stills)

1. Create a free account at <https://www.themoviedb.org>.
2. **Settings → API → Create**, choose **Developer**, fill in the form.
3. Copy the **API Key**. This is `TMDB_KEY`.

TMDB asks you to credit them. Add this line to your Instagram bio or a pinned
post: *"Film images from TMDB. Not endorsed or certified by TMDB."*

### 4. Put the bot on GitHub

1. Create a new **public** repository at <https://github.com/new>. It must be
   public: Instagram downloads each slide from it.
2. Upload everything in this folder, including the hidden `.github` folder.
   If the web uploader skips `.github`, create the two files by hand with
   **Add file → Create new file**, named `.github/workflows/post.yml` and
   `.github/workflows/refresh-token.yml`.
3. **Settings → Secrets and variables → Actions → New repository secret**.
   Add `IG_TOKEN` and `TMDB_KEY`.

### 5. Make it yours

In `config.json`, set `handle` to your Instagram handle. It appears on every
slide.

### 6. Test it

**Actions → Post to Instagram → Run workflow**. After a minute or two the
first carousel should be on your profile. From then on it posts twice a day.

## Check the first few image posts

Painting close-ups are centred on a point I estimated for each picture. If one
misses, change that post's `detail` in `posts.json`.


The bot picks the two highest-rated stills TMDB has for each film. Nobody has
looked at them in advance. If a post gets a frame you don't like, add
`"skip_stills": 2` to that post in `posts.json` to use the next two instead.

## Adding posts

The bot ships with 39 carousels, which is about 19 days at two a day. It posts them
in the order they appear in `posts.json`. When five or fewer are left, the
workflow run shows a warning. When they run out, it falls back to the single
quote cards in `quotes.json` so the account never goes quiet.

To add a post, copy an existing entry in `posts.json` and change it:

| Field | Meaning |
|---|---|
| `id` | Any unique name, lower-case with dashes. |
| `kind` | `film`, `art`, `book` or `music`. Sets the colours and where images come from. |
| `wiki` | Optional. The exact Wikipedia page title, for when the plain title is ambiguous (a film named after a town, say). |
| `cover_title` | Optional. Search for a cover under a different name, e.g. the collection a poem comes from. |
| `image`, `detail` | Art posts only. `{"aic": "Exact title"}` finds the painting at the Art Institute of Chicago; it must be public domain there. `detail` is the point the close-up centres on, as fractions across and down, e.g. `[0.7, 0.4]`. |
| `title`, `year` | For films these must match TMDB so the right stills are found. |
| `director` / `author` / `artist` | Shown under the title. |
| `hook` | The big line on the first slide. |
| `slides` | Each is either `k` (small label), `h` (heading), `b` (body), or a quote: `q` and `by`. Keep bodies under about 55 words. |
| `pairs` | One or two recommendations for the last slide. |
| `caption`, `tags` | The text under the post. |

To see slides before they go out: `pip install -r requirements.txt`, then
`python bot.py preview`. They appear in `preview/`.

## Changing things

| What | Where |
|---|---|
| Post times | The two `cron` lines in `.github/workflows/post.yml`, in UTC. `"7 4 * * *"` is 04:07 UTC daily. |
| Post a specific one now | **Run workflow** and type its `id`. |
| Colours | `THEMES` at the top of `bot.py`. |
| Hashtags, last-slide line | `config.json`. |

## Keeping the Instagram token alive

Tokens expire after 60 days. The **Refresh Instagram token** workflow extends
it twice a month. If that workflow fails with "Instagram issued a new token":

1. GitHub → profile picture → **Settings → Developer settings → Personal
   access tokens → Fine-grained tokens → Generate new token**.
2. Give it access to this repository only, with **Secrets: Read and write**.
3. Add it as a repository secret named `GH_PAT`.

## If a post fails

Open **Actions**, click the failed run, read the last step.

- **"Images are not publicly reachable"**: the repository is private.
- **"Instagram API error … OAuthException"**: the token is wrong or expired.
- **Push rejected**: **Settings → Actions → General → Workflow permissions →
  Read and write permissions**.
- **"TMDB: could not fetch stills"** or **"AIC: no public-domain match"**: not
  a failure. The post went out text-only. Check the `TMDB_KEY` secret, or the
  painting's title in `posts.json`.

## Credits

Fonts are Playfair Display and Inter, both under the SIL Open Font License
(see `fonts/`). This product uses the TMDB API but is not endorsed or
certified by TMDB.
