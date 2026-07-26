# 🎬 Cinelog API (Go Backend)

Backend REST API untuk platform pelacak film dan serial TV Cinelog, dibangun menggunakan **Go (Fiber v2)**, **GORM (PostgreSQL/SQLite)**, dan **TMDB API**.

---

## 🗄️ Skema Database

```text
users (1) ───< user_lists (M) >─── (1) movies
```

- **`users`**: Data akun pengguna (`username`, `email`, `password` hash).
- **`movies`**: Cache metadata media (`tmdb_id`, `media_type`, `title`, `overview`, `local_poster_path`, `local_backdrop_path`, `director`, `cast`, `total_seasons`, `total_episodes`, `next_air_date`, `media_status`).
- **`user_lists`**: Personal watchlist (`user_id`, `movie_id`, `status`, `rating` (1.0 - 5.0 bintang), `favorite`, `notes`, `season_watched`, `episodes_watched`, `total_episodes`).

---

## 📡 Daftar Endpoint API

### 🔑 Autentikasi (`/api/auth`)
- `POST /api/auth/register` - Pendaftaran akun baru
- `POST /api/auth/login` - Login & penerbitan JWT token

### 🎬 Catalog & Detail Media
- `GET /api/search?q=query&type=all|movie|tv` - Search film & TV series dari TMDB
- `GET /api/detail?id=tmdbId&type=movie|tv` - Detail lengkap sutradara, cast, total season, & next air date
- `GET /api/tv/season?id=tmdbId&season=1` - Daftar episode lengkap dalam season tertentu
- `GET /uploads/*` & `GET /api/uploads/*` - Serving file gambar lokal dari storage VPS

### 🔖 Watchlist Management (Protected JWT)
*Header: `Authorization: Bearer <token_jwt>`*
- `GET /api/user/profile` - Verifikasi profil user
- `GET /api/user/watchlist?status=watching&favorite=true&media_type=tv` - Ambil daftar tontonan user
- `POST /api/user/watchlist` - Tambah item ke watchlist & otomatis download poster ke VPS
- `PUT /api/user/watchlist/:id` - Update status, rating 1-5, notes, & episode
- `PUT /api/user/watchlist/:id/progress` - **Inkremen +1 episode** (Gestur TV Time)
- `PUT /api/user/watchlist/:id/set-progress` - Set spesifik season & episode ditonton
- `DELETE /api/user/watchlist/:id` - Hapus item dari watchlist
- `GET /api/user/watchlist/check/:tmdbId?type=movie|tv` - Cek keberadaan item di watchlist

---

## 🚀 Jalankan & Update di VPS

```bash
cd /path/to/cinelog-api
git pull origin main
go build -o cinelog-api main.go
sudo systemctl restart cinelog-api
```
