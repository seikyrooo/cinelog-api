# 🎬 Cinelog API (Go Backend)

Backend REST API untuk platform pelacak film dan serial TV Cinelog, dibangun menggunakan **Go (Fiber v2)**, **GORM (PostgreSQL/SQLite)**, dan **TMDB API**.

---

## 🗄️ Skema Database

```text
users (1) ───< user_lists / user_media_entries (M) >─── (1) movies / media
```

- **`users`**: Data akun pengguna + profile dasar (`username`, `email`, `password` hash, `bio`, `avatar_url`, `is_public`).
- **`movies`** *(domain baru: `media`)*: Cache metadata media (`tmdb_id`, `media_type`, `title`, `original_title`, `overview`, `release_date`, `first_air_date`, `local_poster_path`, `local_backdrop_path`, `genres`, `director`, `cast`, `total_seasons`, `total_episodes`, `next_air_date`, `media_status`).
- **`user_lists`** *(domain baru: `user_media_entries`)*: State personal user terhadap media (`status`, `rating` **0–10**, `favorite`, `notes`, `visibility_rating`, `visibility_favorite`, `started_at`, `completed_at`).
- **`user_tv_progress`**: Fondasi transisi untuk memisahkan progress TV dari personal entry (`current_season`, `current_episode`, `episodes_watched_count`, `last_watched_at`).
- **`user_follows`**: Relasi follow/unfollow antar user untuk social-lite.

### Catatan Contract Transisi

- Naming domain produk mengikuti spec baru: **Media** dan **UserMediaEntry**.
- Untuk menjaga kompatibilitas data dan controller lama, implementasi backend saat ini **masih memetakan domain baru ke tabel legacy** `movies` dan `user_lists`.
- Endpoint watchlist lama masih aktif selama fase transisi, sambil kontrak baru `/api/me/library` disiapkan bertahap.
- Rating personal user sudah distandardkan ke skala **0–10**.

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
- `PUT /api/user/watchlist/:id` - Update status, rating 0-10, notes, visibility, & episode
- `PUT /api/user/watchlist/:id/progress` - **Inkremen +1 episode** (Gestur TV Time)
- `PUT /api/user/watchlist/:id/set-progress` - Set spesifik season & episode ditonton
- `DELETE /api/user/watchlist/:id` - Hapus item dari watchlist
- `GET /api/user/watchlist/check/:tmdbId?type=movie|tv` - Cek keberadaan item di watchlist

---

## 🧹 Database Cleansing & Maintenance Tool

Tool CLI pembersih dan normalisasi database yang aman untuk database production (dilengkapi fitur **Dry-Run** dan **Transactional Rollback**).

### Fitur Pembersihan:
1. **Normalisasi Status**: Memperbaiki data status yang tidak valid atau null menjadi `plan_to_watch`.
2. **Clamp Rating**: Menjaga seluruh rating user berada di rentang valid **0.0 – 10.0**.
3. **Standarisasi Visibility**: Memastikan nilai visibilitas rating dan favorit default ke `public`.
4. **Pembersihan Data Orphan**: Menghapus baris `user_lists`, `user_tv_progress`, dan `user_follows` yang kehilangan relasi parentnya.
5. **Sinkronisasi Progress**: Mengoreksi hitungan episode tontonan yang telah berstatus `completed`.
6. **Prune Media Tak Terpakai** *(opsional)*: Membersihkan cache TMDB di tabel `movies` yang tidak ada di daftar user manapun.

### Cara Menjalankan:

```bash
# 1. Preview saja tanpa mengubah data (Aman / Dry-Run)
go run cmd/cleanse/main.go

# 2. Eksekusi pembersihan ke database (Commit Transaction)
go run cmd/cleanse/main.go -apply

# 3. Eksekusi pembersihan sekaligus prune cache film yang tidak dipakai siapapun
go run cmd/cleanse/main.go -apply -prune-unused-media

# 4. (Khusus Dev/Testing) Hard Reset / Truncate seluruh tabel
go run cmd/cleanse/main.go -reset -confirm=CONFIRM_RESET
```

---

## 🚀 Jalankan & Update di VPS

```bash
cd /path/to/cinelog-api
git pull origin main
go build -o cinelog-api main.go
sudo systemctl restart cinelog-api
```
