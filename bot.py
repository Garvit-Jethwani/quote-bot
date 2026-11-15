#!/usr/bin/env python3
"""Daily carousel bot for Instagram: scene studies, shelf notes, liner notes.

  python bot.py render [--id POST_ID]     pick the next post and draw its slides
  python bot.py publish                   publish what `render` drew
  python bot.py preview [--id POST_ID]    draw every post into preview/
  python bot.py refresh-token             extend the Instagram token

Environment:
  IG_TOKEN     Instagram access token (required to publish)
  IG_USER_ID   optional; looked up from the token when missing
  TMDB_KEY     optional; a TMDB API key or read-access token. With it, film
               posts get real stills. Without it they get a poster instead.
  NO_IMAGES    set to 1 to draw every post text-only.
Other images (posters, album and book covers, paintings) need no key.
"""
import argparse
import colorsys
import datetime as dt
import hashlib
import io
import json
import os
import random
import subprocess
import sys
import time
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont, ImageOps

ROOT = Path(__file__).resolve().parent
OUT = ROOT / "out"
PENDING = ROOT / "pending.json"
POSTED = ROOT / "posted.json"
TMDB_API = os.environ.get("TMDB_API_BASE", "https://api.themoviedb.org/3").rstrip("/")
TMDB_IMG = os.environ.get("TMDB_IMG_BASE", "https://image.tmdb.org/t/p/w1280").rstrip("/")
AIC_API = os.environ.get("AIC_API_BASE", "https://api.artic.edu/api/v1").rstrip("/")
AIC_IMG = os.environ.get("AIC_IMG_BASE", "https://www.artic.edu/iiif/2").rstrip("/")
ITUNES_API = os.environ.get("ITUNES_API_BASE", "https://itunes.apple.com/search")
OPENLIB_API = os.environ.get("OPENLIB_API_BASE", "https://openlibrary.org").rstrip("/")
OPENLIB_IMG = os.environ.get("OPENLIB_IMG_BASE", "https://covers.openlibrary.org").rstrip("/")
WIKI_API = os.environ.get("WIKI_API_BASE", "https://en.wikipedia.org/api/rest_v1").rstrip("/")

W, H = 1080, 1350          # Instagram portrait (4:5)
M = 96                     # side margin
KEEP_IMAGES_DAYS = 14

THEMES = {  # background, text, accent, muted
    "film":  ("#0F0F11", "#EFEAE0", "#E0A458", "#8D8A84"),
    "book":  ("#F1EADB", "#1E1B17", "#A8472A", "#7B7266"),
    "music": ("#15171C", "#ECE9E2", "#7FB8A4", "#8A8F9C"),
    "art":   ("#F5F3EE", "#17171A", "#2F4E7A", "#807C74"),
}
QUOTE_PALETTES = [
    ("#14213D", "#F7F3E9", "#FCA311", "#9AA5BD"),
    ("#F4EDE1", "#1F1B16", "#B5532B", "#7A7166"),
    ("#1E3A2F", "#F1EBDD", "#D9A441", "#93AB9E"),
    ("#4A1525", "#F6E9E0", "#E0A458", "#C098A2"),
    ("#1B1B1E", "#EDEDED", "#E4572E", "#8C8C92"),
    ("#E9DCC9", "#2B2A28", "#2F5D62", "#7D7468"),
]


# ---------------------------------------------------------------- data

def load_json(path, default=None):
    if not path.exists():
        return default
    return json.loads(path.read_text(encoding="utf-8"))


def quote_id(q):
    return hashlib.sha1(q["text"].encode("utf-8")).hexdigest()[:10]


def next_post(posted, wanted=None):
    posts = load_json(ROOT / "posts.json", [])
    if wanted:
        match = [p for p in posts if p["id"] == wanted]
        return match[0] if match else sys.exit(f"No post with id {wanted!r} in posts.json")
    used = {p["id"] for p in posted}
    return next((p for p in posts if p["id"] not in used), None)


def pick_quote(posted):
    quotes = load_json(ROOT / "quotes.json")
    last = next((p["type"] for p in reversed(posted) if p["type"] in ("book", "movie")), "movie")
    kind = "movie" if last == "book" else "book"
    pool = [q for q in quotes if q["type"] == kind] or quotes
    used = {p["id"] for p in posted}
    fresh = [q for q in pool if quote_id(q) not in used]
    if not fresh:  # every quote used: start again with the least recent half
        order = {p["id"]: i for i, p in enumerate(posted)}
        pool.sort(key=lambda q: order.get(quote_id(q), -1))
        fresh = pool[: max(1, len(pool) // 2)]
    return random.choice(fresh)


def hashtags(cfg, *groups):
    """Specific tags first, then broad ones; no repeats; capped (Instagram favours a few precise tags)."""
    seen, out = set(), []
    for tag in " ".join(g for g in groups if g).split():
        tag = tag if tag.startswith("#") else f"#{tag}"
        if tag.lower() not in seen:
            seen.add(tag.lower())
            out.append(tag)
    return " ".join(out[: cfg.get("max_hashtags", 5)])


def post_caption(p, cfg, image_credit):
    parts = [p["caption"]]
    pairs = [f"{x['label'].lower()}: {x['title']} by {x['by']}"
             for x in p.get("pairs", []) if x.get("caption", True)]
    if pairs:
        parts.append("\n".join(pairs))
    if p["kind"] == "film":
        credit = f"{p['title']} ({p['year']}), dir. {p['director']}."
        parts.append(f"{credit} {image_credit}".strip())
    if p["kind"] == "art":
        credit = f"{p['artist']}, {p['title']} ({p['year']})."
        parts.append(f"{credit} {image_credit}".strip())
    if p["kind"] in ("book", "music") and image_credit:
        parts.append(image_credit)
    parts.append(cfg.get("caption_footer", ""))
    parts.append(hashtags(cfg, p.get("tags", ""), cfg["hashtags"].get(p["kind"], "")))
    return "\n\n".join(x for x in parts if x)[:2200]


def quote_caption(q, cfg):
    credit = f"{q['source']} ({q['by']})" if q["type"] == "movie" else f"{q['by']}, {q['source']}"
    text = q["text"] if q["text"].startswith("“") else f"“{q['text']}”"
    parts = [text, credit, cfg.get("caption_footer", ""), hashtags(cfg, cfg["hashtags"].get("book_quote" if q["type"] == "book" else q["type"], ""))]
    return "\n\n".join(x for x in parts if x)[:2200]


# ---------------------------------------------------------------- images
# Every post tries several sources in turn and uses the first that answers:
#   film   TMDB stills -> TMDB poster -> Apple poster -> Wikipedia
#   art    Art Institute of Chicago -> Wikipedia
#   music  Apple album cover -> Wikipedia
#   book   Open Library cover -> Wikipedia
# A source that fails is skipped; a post with no image is drawn text-only.

UA = {"User-Agent": "carousel-bot/1.0", "AIC-User-Agent": "carousel-bot"}
_tmdb_cache = {}


def http_json(url, headers=None, **params):
    import requests
    r = requests.get(url, params=params, headers={**UA, **(headers or {})}, timeout=30)
    if r.status_code != 200:
        raise RuntimeError(f"HTTP {r.status_code}")
    return r.json()


def http_image(url, min_side=200):
    import requests
    r = requests.get(url, headers=UA, timeout=60)
    if r.status_code != 200:
        return None
    im = Image.open(io.BytesIO(r.content)).convert("RGB")
    return im if min(im.size) >= min_side else None


def creator(post):
    return post.get("director") or post.get("author") or post.get("artist") or ""


def same(a, b):
    """Loose title match: one contains the other, ignoring case and punctuation."""
    norm = lambda t: "".join(c for c in (t or "").lower() if c.isalnum() or c == " ").strip()
    a, b = norm(a), norm(b)
    return bool(a and b) and (a in b or b in a)


def tmdb_get(path, **params):
    key = os.environ["TMDB_KEY"].strip()
    headers = {}
    if len(key) > 40:                      # v4 read-access token
        headers["Authorization"] = f"Bearer {key}"
    else:                                  # v3 API key
        params["api_key"] = key
    return http_json(f"{TMDB_API}/{path}", headers, **params)


def tmdb_movie(post):
    if post["id"] not in _tmdb_cache:
        found = tmdb_get("search/movie", query=post["title"], year=post["year"]).get("results", [])
        if not found:
            found = tmdb_get("search/movie", query=post["title"]).get("results", [])
        want = post["title"].lower()
        exact = [m for m in found if want in (m.get("title", "").lower(), m.get("original_title", "").lower())]
        _tmdb_cache[post["id"]] = (exact or found or [None])[0]
    return _tmdb_cache[post["id"]]


def src_tmdb_stills(post):
    if not os.environ.get("TMDB_KEY"):
        return [], ""
    movie = tmdb_movie(post)
    if not movie:
        return [], ""
    shots = tmdb_get(f"movie/{movie['id']}/images", include_image_language="null").get("backdrops", [])
    shots = [x for x in shots if x.get("iso_639_1") in (None, "xx") and x.get("width", 0) >= 1280]
    shots.sort(key=lambda x: (x.get("vote_average", 0), x.get("vote_count", 0)), reverse=True)
    skip = post.get("skip_stills", 0)          # set in posts.json to pick different frames
    images = [im for im in (http_image(f"{TMDB_IMG}{x['file_path']}") for x in shots[skip: skip + 2]) if im]
    return images, "Stills via TMDB."


def src_tmdb_poster(post):
    if not os.environ.get("TMDB_KEY"):
        return [], ""
    movie = tmdb_movie(post)
    if not movie or not movie.get("poster_path"):
        return [], ""
    im = http_image(f"{TMDB_IMG}{movie['poster_path']}")
    return ([im] if im else []), "Poster via TMDB."


def apple(post, entity, name_key):
    term = f"{post.get('cover_title', post['title'])} {creator(post)}".strip()
    hits = http_json(ITUNES_API, term=term, entity=entity, limit=10).get("results", [])
    first = creator(post).split()[0] if creator(post) else ""
    for h in hits:
        if not same(h.get(name_key), post.get("cover_title", post["title"])) or not h.get("artworkUrl100"):
            continue
        if entity == "album" and first and first.lower() not in h.get("artistName", "").lower():
            continue
        if entity == "movie" and str(post["year"]) not in (h.get("releaseDate") or "")[:4]:
            continue
        im = http_image(h["artworkUrl100"].replace("100x100bb", "1200x1200bb"))
        if im:
            return [im]
    return []


def src_apple_movie(post):
    return apple(post, "movie", "trackName"), "Poster via Apple TV."


def src_apple_album(post):
    return apple(post, "album", "collectionName"), "Cover via Apple Music."


def src_openlibrary(post):
    title = post.get("cover_title", post["title"])
    docs = http_json(f"{OPENLIB_API}/search.json", title=title, author=creator(post),
                     limit=10, fields="title,author_name,cover_i").get("docs", [])
    for doc in docs:
        if doc.get("cover_i") and same(doc.get("title"), title):
            im = http_image(f"{OPENLIB_IMG}/b/id/{doc['cover_i']}-L.jpg")
            if im:
                return [im], "Cover via Open Library."
    return [], ""


def src_aic(post):
    want = (post.get("image") or {}).get("aic")
    if not want:
        return [], ""
    hits = http_json(f"{AIC_API}/artworks/search", q=want, limit=15,
                     fields="id,title,image_id,is_public_domain,artist_title").get("data", [])
    surname = post["artist"].split()[-1].lower()
    for hit in hits:
        if (hit.get("image_id") and hit.get("is_public_domain")
                and want.lower() in (hit.get("title") or "").lower()
                and surname in (hit.get("artist_title") or "").lower()):
            for size in ("1686,", "843,"):
                im = http_image(f"{AIC_IMG}/{hit['image_id']}/full/{size}/0/default.jpg")
                if im:
                    return [im], "Image: Art Institute of Chicago, public domain."
    return [], ""


WIKI_WORDS = {"film": ("film",), "music": ("album",), "art": ("painting", "print", "woodblock"),
              "book": ("novel", "book", "poem", "letters", "collection", "work by", "novella")}


def src_wikipedia(post):
    from urllib.parse import quote
    t, y = post["title"], post["year"]
    names = [post["wiki"]] if post.get("wiki") else {
        "film": [f"{t} ({y} film)", f"{t} (film)", t],
        "music": [f"{t} ({creator(post)} album)", f"{t} (album)", t],
        "book": [f"{t} (novel)", f"{t} (book)", t, post.get("cover_title", t)],
        "art": [f"{t} (painting)", t, (post.get("image") or {}).get("wiki", t)],
    }[post["kind"]]
    for name in names:
        try:
            page = http_json(f"{WIKI_API}/page/summary/{quote(name.replace(' ', '_'), safe='')}")
        except RuntimeError:
            continue
        about = (page.get("description") or "").lower()
        if page.get("type") != "standard" or not any(w in about for w in WIKI_WORDS[post["kind"]]):
            continue                           # wrong page (a town, a person, a disambiguation)
        url = (page.get("originalimage") or page.get("thumbnail") or {}).get("source")
        im = http_image(url) if url else None
        if im:
            return [im], "Image via Wikipedia."
    return [], ""


SOURCES = {
    "film": [src_tmdb_stills, src_tmdb_poster, src_apple_movie, src_wikipedia],
    "art": [src_aic, src_wikipedia],
    "music": [src_apple_album, src_wikipedia],
    "book": [src_openlibrary, src_wikipedia],
}


def fetch_images(post):
    """Return (images, credit). Tries each source in turn; never raises."""
    if os.environ.get("NO_IMAGES"):
        return [], ""
    for source in SOURCES[post["kind"]]:
        name = source.__name__[4:]
        try:
            images, credit = source(post)
        except Exception as e:                  # a missing image must never stop a post
            print(f"images: {name} failed for {post['id']} ({str(e)[:90]})")
            continue
        if images:
            print(f"images: {post['id']} <- {name} ({len(images)})")
            return images, credit
    print(f"images: nothing found for {post['id']}; drawing text-only slides")
    return [], ""


def detail_of(img, focus, w, h, zoom=2.3):
    """Crop a close-up around `focus` (x, y as fractions) at the w:h shape."""
    cw = img.width / zoom
    ch = cw * h / w
    if ch > img.height:
        ch = img.height
        cw = ch * w / h
    x = min(max(focus[0] * img.width - cw / 2, 0), img.width - cw)
    y = min(max(focus[1] * img.height - ch / 2, 0), img.height - ch)
    return img.crop((int(x), int(y), int(x + cw), int(y + ch))).resize((w, h), Image.LANCZOS)


def mix(c1, c2, t):
    a, b = (tuple(int(c[i:i + 2], 16) for i in (1, 3, 5)) for c in (c1, c2))
    return "#%02X%02X%02X" % tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(3))


def accent_from(img, fallback):
    """Pick a readable accent colour from the still's dominant hue."""
    bins = {}
    px = img.convert("RGB").resize((48, 27)).tobytes()
    for i in range(0, len(px), 3):
        r, g, b = px[i], px[i + 1], px[i + 2]
        h, s, v = colorsys.rgb_to_hsv(r / 255, g / 255, b / 255)
        if s > 0.3 and 0.25 < v < 0.95:
            bins.setdefault(int(h * 12), []).append(h)
    if not bins:
        return fallback
    hues = max(bins.values(), key=len)
    if len(hues) < 40:
        return fallback
    r, g, b = colorsys.hsv_to_rgb(sum(hues) / len(hues), 0.5, 0.93)
    return "#%02X%02X%02X" % (int(r * 255), int(g * 255), int(b * 255))


# ---------------------------------------------------------------- drawing helpers

def font(name, size, weight=None):
    f = ImageFont.truetype(str(ROOT / "fonts" / f"{name}.ttf"), size)
    if weight is not None:
        axes = []
        for a in f.get_variation_axes():
            tag = a["name"].decode() if isinstance(a["name"], bytes) else a["name"]
            if tag.lower() == "weight":
                axes.append(weight)
            elif tag.lower() == "optical size":
                axes.append(min(a["maximum"], max(a["minimum"], size)))
            else:
                axes.append(a["default"])
        f.set_variation_by_axes(axes)
    return f


def wrap(d, text, fnt, width):
    out = []
    for para in text.split("\n"):
        line = ""
        for word in para.split():
            trial = f"{line} {word}".strip()
            if line and d.textlength(trial, font=fnt) > width:
                out.append(line)
                line = word
            else:
                line = trial
        out.append(line)
    return out


def fit(d, text, name, weight, width, max_h, hi, lo, ratio):
    """Largest font size between hi and lo whose wrapped text fits max_h."""
    for size in range(hi, lo - 1, -2):
        f = font(name, size, weight)
        lines = wrap(d, text, f, width)
        lh = int(size * ratio)
        if len(lines) * lh <= max_h and max(d.textlength(l, font=f) for l in lines) <= width:
            break
    return f, lines, lh


def para(d, x, y, lines, fnt, fill, lh):
    for line in lines:
        d.text((x, y), line, font=fnt, fill=fill)
        y += lh
    return y


def tracked(d, xy, text, fnt, fill, spacing):
    x, y = xy
    for ch in text:
        d.text((x, y), ch, font=fnt, fill=fill)
        x += d.textlength(ch, font=fnt) + spacing
    return x


def tracked_w(d, text, fnt, spacing):
    return sum(d.textlength(c, font=fnt) + spacing for c in text) - spacing


def canvas(th):
    img = Image.new("RGB", (W, H), th[0])
    return img, ImageDraw.Draw(img)


def chrome(d, th, label, n, total, handle, hint=""):
    """Header (label, rule, page count) and footer (hint, handle)."""
    bg, fg, accent, muted = th
    f = font("Inter", 24, 600)
    end = tracked(d, (M, 78), label, f, accent, 6)
    count = f"{n:02d} / {total:02d}"
    cw = tracked_w(d, count, f, 3)
    tracked(d, (W - M - cw, 78), count, f, muted, 3)
    d.line([(end + 22, 93), (W - M - cw - 22, 93)], fill=muted, width=1)
    fy = H - 104
    if hint:
        e = tracked(d, (M, fy), hint, f, accent, 6)
        d.line([(e + 14, fy + 15), (e + 74, fy + 15)], fill=accent, width=3)
        d.polygon([(e + 74, fy + 8), (e + 86, fy + 15), (e + 74, fy + 22)], fill=accent)
    else:
        d.line([(M, fy + 15), (M + 60, fy + 15)], fill=accent, width=3)
    if handle:
        hf = font("Inter", 24, 500)
        tracked(d, (W - M - tracked_w(d, handle, hf, 2), fy), handle, hf, muted, 2)
    return fy


def save(img, path):
    path.parent.mkdir(parents=True, exist_ok=True)
    img.save(path, "JPEG", quality=92, optimize=True)
    return path


# ---------------------------------------------------------------- carousel slides

def byline(p):
    who = p.get("director") or p.get("author") or p.get("artist") or ""  # artist: music and art
    return f"{who} · {p['year']}".upper() if who else str(p["year"])


def slide_cover(p, th, still, total, cfg):
    bg, fg, accent, muted = th
    img, d = canvas(th)
    fy = chrome(d, th, p["label"], 1, total, cfg.get("handle", ""), cfg.get("swipe_hint", ""))
    width = W - 2 * M
    title_f = font("PlayfairDisplay-Italic", 42, 500)
    by_f = font("Inter", 25, 500)
    title_lines = wrap(d, p["title"], title_f, width)
    meta_h = 3 + 36 + len(title_lines) * 54 + 12 + 30

    if still is not None:                       # frame kept at 16:9, text underneath
        if p["kind"] == "art" or still.width / still.height < 1.5:
            # paintings, posters and covers are shown whole on a panel, never cropped
            d.rectangle([0, 150, W, 150 + 608], fill=mix(bg, fg, 0.09))
            box = (min(W - 80, still.width * 2), min(608 - 56, still.height * 2))
            fitted = ImageOps.contain(still, box, Image.LANCZOS)
            img.paste(fitted, ((W - fitted.width) // 2, 150 + (608 - fitted.height) // 2))
        else:
            img.paste(ImageOps.fit(still, (W, 608), Image.LANCZOS), (0, 150))
        top, bottom = 150 + 608 + 64, fy - 56
        hook_f, hook, lh = fit(d, p["hook"], "PlayfairDisplay", 600, width, bottom - top - meta_h - 40, 76, 40, 1.2)
        y = top
    else:                                       # typographic cover
        top, bottom = 190, fy - 56
        hook_f, hook, lh = fit(d, p["hook"], "PlayfairDisplay", 600, width, bottom - top - meta_h - 60, 112, 48, 1.16)
        y = top + (bottom - top - (len(hook) * lh + 60 + meta_h)) // 2
    y = para(d, M, y, hook, hook_f, fg, lh) + (40 if still is not None else 60)
    d.line([(M, y), (M + 84, y)], fill=accent, width=3)
    y = para(d, M, y + 36, title_lines, title_f, fg, 54) + 12
    tracked(d, (M, y), byline(p), by_f, muted, 4)
    return img


def slide_body(p, th, s, n, total, cfg, still=None):
    bg, fg, accent, muted = th
    img, d = canvas(th)
    fy = chrome(d, th, p["label"], n, total, cfg.get("handle", ""))
    width = W - 2 * M
    top = 190
    if still is not None:
        img.paste(ImageOps.fit(still, (W, 420), Image.LANCZOS, centering=(0.5, 0.35)), (0, 150))
        top = 150 + 420 + 64
    bottom = fy - 60
    kick_f = font("Inter", 24, 600)
    head_f = font("PlayfairDisplay", 70 if still is None else 56, 600)
    head = wrap(d, s["h"], head_f, width)
    head_lh = int(head_f.size * 1.18)
    fixed = 30 + 40 + len(head) * head_lh + 48
    body_f, body, lh = fit(d, s["b"], "Inter", 400, width, bottom - top - fixed, 48 if still is None else 40, 28, 1.5)
    y = top if still is not None else top + max(0, (bottom - top - fixed - len(body) * lh) * 2 // 5)
    tracked(d, (M, y), f"{n - 1:02d}  {s['k']}", kick_f, accent, 5)
    y = para(d, M, y + 30 + 40, head, head_f, fg, head_lh) + 48
    para(d, M, y, body, body_f, fg, lh)
    return img


def slide_quote(p, th, s, n, total, cfg):
    bg, fg, accent, muted = th
    img, d = canvas(th)
    fy = chrome(d, th, p["label"], n, total, cfg.get("handle", ""))
    width = W - 2 * M
    by_f = font("Inter", 25, 500)
    by = wrap(d, s.get("by", "").upper(), by_f, width - 40)
    top, bottom = 190, fy - 60
    q_f, lines, lh = fit(d, s["q"], "PlayfairDisplay-Italic", 500, width, bottom - top - 150 - 100 - len(by) * 38, 78, 40, 1.3)
    block = 150 + len(lines) * lh + 56 + 40 + len(by) * 38
    y = top + (bottom - top - block) // 2
    d.text((M - 8, y - 70), "“", font=font("PlayfairDisplay", 300, 700), fill=accent)
    y = para(d, M, y + 150, lines, q_f, fg, lh) + 56
    d.line([(M, y), (M + 84, y)], fill=accent, width=3)
    y += 40
    for line in by:
        tracked(d, (M, y), line, by_f, muted, 4)
        y += 38
    return img


def slide_pairs(p, th, n, total, cfg):
    bg, fg, accent, muted = th
    img, d = canvas(th)
    fy = chrome(d, th, p["label"], n, total, cfg.get("handle", ""))
    width = W - 2 * M
    kick_f, by_f, note_f = font("Inter", 24, 600), font("Inter", 25, 500), font("Inter", 37, 400)
    blocks = []
    for x in p["pairs"]:
        t_f, t, t_lh = fit(d, x["title"], "PlayfairDisplay", 600, width, 240, 68, 42, 1.18)
        by = wrap(d, x["by"].upper(), by_f, width - 60)
        note = wrap(d, x.get("note", ""), note_f, width)
        h = 54 + len(t) * t_lh + 16 + len(by) * 38 + 26 + len(note) * 55
        blocks.append((x, t_f, t, t_lh, by, note, h))
    closing = cfg.get("closing_line", "")
    gap = 84
    total_h = sum(b[-1] for b in blocks) + gap * (len(blocks) - 1) + (110 if closing else 0)
    y = 190 + max(0, (fy - 60 - 190 - total_h) // 2)
    for i, (x, t_f, t, t_lh, by, note, h) in enumerate(blocks):
        tracked(d, (M, y), x["label"], kick_f, accent, 5)
        y = para(d, M, y + 54, t, t_f, fg, t_lh) + 16
        for line in by:
            tracked(d, (M, y), line, by_f, muted, 4)
            y += 38
        y = para(d, M, y + 26, note, note_f, fg, 55)
        if i < len(blocks) - 1:
            d.line([(M, y + gap // 2), (W - M, y + gap // 2)], fill=muted, width=1)
            y += gap
    if closing:
        d.text((M, y + 66), closing, font=font("PlayfairDisplay-Italic", 34, 500), fill=muted)
    return img


def render_post(p, cfg, folder, stem):
    th = list(THEMES[p["kind"]])
    stills, credit = fetch_images(p)
    if stills and p["kind"] == "art" and stills[0].width >= 1200:
        stills.append(detail_of(stills[0], p.get("detail", (0.5, 0.5)), W, 420))   # close-up for slide 2
    if stills and p["kind"] == "film":
        th[2] = accent_from(stills[0], th[2])
        if len(stills) > 1 and stills[1].width / stills[1].height < 1.5:
            stills = stills[:1]
    total = 1 + len(p["slides"]) + (1 if p.get("pairs") else 0)
    images = [slide_cover(p, th, stills[0] if stills else None, total, cfg)]
    for i, s in enumerate(p["slides"], start=2):
        if "q" in s:
            images.append(slide_quote(p, th, s, i, total, cfg))
        else:
            images.append(slide_body(p, th, s, i, total, cfg, stills[1] if i == 2 and len(stills) > 1 else None))
    if p.get("pairs"):
        images.append(slide_pairs(p, th, total, total, cfg))
    paths = [save(im, folder / f"{stem}-{i:02d}.jpg") for i, im in enumerate(images, start=1)]
    return paths, credit


# ---------------------------------------------------------------- single quote card

def render_quote(q, cfg, path):
    th = QUOTE_PALETTES[int(quote_id(q), 16) % len(QUOTE_PALETTES)]
    bg, fg, accent, muted = th
    img, d = canvas(th)
    fy = chrome(d, th, cfg["labels"].get(q["type"], "QUOTE"), 1, 1, cfg.get("handle", ""))
    width = W - 2 * M
    src_f, by_f = font("PlayfairDisplay-Italic", 44, 500), font("Inter", 27, 500)
    src = wrap(d, q["source"], src_f, width)
    attr_h = 3 + 40 + len(src) * 58 + 14 + 34
    top, bottom = 190, fy - 60
    q_f, lines, lh = fit(d, q["text"], "PlayfairDisplay", 500, width, bottom - top - 150 - attr_h - 60, 96, 40, 1.28)
    y = top + (bottom - top - (150 + len(lines) * lh + 60 + attr_h)) // 2
    d.text((M - 8, y - 70), "“", font=font("PlayfairDisplay", 300, 700), fill=accent)
    y = para(d, M, y + 150, lines, q_f, fg, lh) + 60
    d.line([(M, y), (M + 90, y)], fill=accent, width=3)
    y = para(d, M, y + 40, src, src_f, fg, 58) + 14
    tracked(d, (M, y), str(q["by"]).upper(), by_f, muted, 4)
    return save(img, path)


# ---------------------------------------------------------------- commands

def cmd_render(args):
    cfg = load_json(ROOT / "config.json")
    posted = load_json(POSTED, [])
    now = dt.datetime.now(dt.timezone.utc)
    post = next_post(posted, args.id)
    if post:
        paths, credit = render_post(post, cfg, OUT, f"{now:%Y%m%d-%H%M}-{post['id']}")
        entry = {"id": post["id"], "type": "post", "date": f"{now:%Y-%m-%d}"}
        caption, what = post_caption(post, cfg, credit), f"{post['title']} ({len(paths)} slides)"
        left = sum(1 for p in load_json(ROOT / "posts.json", []) if p["id"] not in {x["id"] for x in posted}) - 1
        if left <= 6:
            print(f"::warning::Only {max(left, 0)} unposted carousels left in posts.json. Add more soon.")
    else:                                       # carousels used up: fall back to quote cards
        q = pick_quote(posted)
        paths = [render_quote(q, cfg, OUT / f"{now:%Y%m%d-%H%M}-{quote_id(q)}.jpg")]
        entry = {"id": quote_id(q), "type": q["type"], "date": f"{now:%Y-%m-%d}"}
        caption, what = quote_caption(q, cfg), f"quote from {q['source']}"

    cutoff = f"{now - dt.timedelta(days=KEEP_IMAGES_DAYS):%Y%m%d}"
    for old in OUT.glob("*.jpg"):               # drop old slides so the repo stays small
        if old.name[:8] < cutoff:
            old.unlink()

    posted.append(entry)
    POSTED.write_text(json.dumps(posted, indent=0) + "\n", encoding="utf-8")
    PENDING.write_text(json.dumps(
        {"images": [p.relative_to(ROOT).as_posix() for p in paths], "caption": caption},
        ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"Drew {what}")


def facebook_token():
    """Tokens from Meta's 'Unified login' / Facebook login start with EAA and use graph.facebook.com."""
    return os.environ.get("IG_TOKEN", "").strip().startswith("EAA")


def api_base():
    if os.environ.get("IG_API_BASE"):
        return os.environ["IG_API_BASE"].rstrip("/")
    return "https://graph.facebook.com" if facebook_token() else "https://graph.instagram.com"


def ig_user():
    if os.environ.get("IG_USER_ID"):
        return os.environ["IG_USER_ID"].strip()
    if not facebook_token():
        return api("GET", "me", fields="user_id")["user_id"]
    try:                                        # look for an Instagram account linked to a Page
        pages = api("GET", "me/accounts", fields="instagram_business_account").get("data", [])
        ids = [p["instagram_business_account"]["id"] for p in pages if p.get("instagram_business_account")]
        if ids:
            return ids[0]
    except SystemExit:
        pass
    sys.exit("Could not work out the Instagram account ID from this token.\n"
             "Add a repository secret named IG_USER_ID with the Instagram account ID\n"
             "shown next to your account on the Meta developer dashboard.")


def api(method, path, **params):
    import requests
    params["access_token"] = os.environ["IG_TOKEN"].strip()
    kw = {"params": params} if method == "GET" else {"data": params}
    r = requests.request(method, f"{api_base()}/{path}", timeout=60, **kw)
    try:
        body = r.json()
    except ValueError:
        body = {"raw": r.text[:500]}
    if r.status_code >= 400 or "error" in body:
        sys.exit(f"Instagram API error on {method} /{path}: "
                 f"HTTP {r.status_code} {json.dumps(body.get('error', body))}")
    return body


def public_url(rel_path):
    if os.environ.get("IMAGE_BASE_URL"):
        return f"{os.environ['IMAGE_BASE_URL'].rstrip('/')}/{rel_path}"
    repo = os.environ.get("GITHUB_REPOSITORY")
    if not repo:
        sys.exit("Set GITHUB_REPOSITORY (owner/repo) or IMAGE_BASE_URL.")
    sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    return f"https://raw.githubusercontent.com/{repo}/{sha}/{rel_path}"


def wait_ready(container):
    for _ in range(40):                         # Instagram downloads and processes the image
        status = api("GET", container, fields="status_code").get("status_code")
        if status == "FINISHED":
            return
        if status in ("ERROR", "EXPIRED"):
            sys.exit(f"Instagram could not process an image (status {status}).")
        time.sleep(3)
    sys.exit("Instagram took too long to process an image.")


def cmd_publish(args):
    import requests
    if not os.environ.get("IG_TOKEN"):
        sys.exit("IG_TOKEN is not set. Add it as a repository secret.")
    pending = load_json(PENDING)
    if not pending:
        sys.exit("Nothing to publish: run `python bot.py render` first.")
    urls = [public_url(p) for p in pending["images"]]

    for _ in range(30):                         # wait until the pushed images are public
        try:
            if all(requests.head(u, timeout=20, allow_redirects=True).status_code == 200 for u in urls):
                break
        except requests.RequestException:
            pass
        time.sleep(4)
    else:
        sys.exit(f"Images are not publicly reachable, e.g. {urls[0]}\n"
                 "Is the repository public and were the images pushed?")

    user = ig_user()
    if len(urls) == 1:
        container = api("POST", f"{user}/media", image_url=urls[0], caption=pending["caption"])["id"]
    else:
        children = []
        for u in urls:
            child = api("POST", f"{user}/media", image_url=u, is_carousel_item="true")["id"]
            wait_ready(child)
            children.append(child)
        container = api("POST", f"{user}/media", media_type="CAROUSEL",
                        children=",".join(children), caption=pending["caption"])["id"]
    wait_ready(container)
    media = api("POST", f"{user}/media_publish", creation_id=container)
    PENDING.unlink()
    print(f"Published {len(urls)} slide(s). Media id: {media['id']}")


def cmd_preview(args):
    cfg = load_json(ROOT / "config.json")
    posts = load_json(ROOT / "posts.json", [])
    if args.id:
        posts = [p for p in posts if p["id"] == args.id] or sys.exit(f"No post with id {args.id!r}")
    for p in posts:
        render_post(p, cfg, ROOT / "preview" / p["id"], p["id"])
    print(f"Drew {len(posts)} carousel(s) into preview/")


def cmd_refresh(args):
    old = os.environ.get("IG_TOKEN") or sys.exit("IG_TOKEN is not set.")
    if facebook_token():
        print("This kind of token is managed on the Meta dashboard and is not refreshed here. Nothing to do.")
        return
    body = api("GET", "refresh_access_token", grant_type="ig_refresh_token")
    new = body["access_token"]
    print(f"::add-mask::{new}")
    print(f"Token is valid for about {body.get('expires_in', 0) // 86400} more days.")
    if new == old:
        return
    if not os.environ.get("GH_PAT"):
        sys.exit("Instagram issued a new token, but there is no GH_PAT secret to save it "
                 "with. See README: 'Keeping the token alive'.")
    env = dict(os.environ, GH_TOKEN=os.environ["GH_PAT"])
    subprocess.run(["gh", "secret", "set", "IG_TOKEN", "--repo", os.environ["GITHUB_REPOSITORY"]],
                   input=new, text=True, env=env, check=True)
    print("Saved the new token as the IG_TOKEN secret.")


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawTextHelpFormatter)
    sub = p.add_subparsers(dest="cmd", required=True)
    r = sub.add_parser("render")
    r.add_argument("--id", help="post id from posts.json (default: next unposted)")
    r.set_defaults(fn=cmd_render)
    sub.add_parser("publish").set_defaults(fn=cmd_publish)
    v = sub.add_parser("preview")
    v.add_argument("--id", help="only this post")
    v.set_defaults(fn=cmd_preview)
    sub.add_parser("refresh-token").set_defaults(fn=cmd_refresh)
    args = p.parse_args()
    args.fn(args)


if __name__ == "__main__":
    main()
