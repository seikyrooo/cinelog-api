# Cinelog API (Go Backend)

Backend REST API untuk aplikasi tracker film dan serial TV Cinelog, dibangun menggunakan **Go (Fiber v2)**, **GORM (PostgreSQL)**, dan **TMDB API**.

---

## Prasyarat
- **Go**: v1.22 atau lebih baru
- **PostgreSQL**: v14 atau lebih baru
- **TMDB API Key**: Dapatkan dari [TheMovieDB.org](https://www.themoviedb.org/settings/api)

---

## Konfigurasi Environment (`.env`)
Salin `.env.example` menjadi `.env`:
```bash
cp .env.example .env
```
Isi variabel berikut:
```env
PORT=3000
DB_DSN=host=localhost user=postgres password=password_db dbname=cinelog port=5432 sslmode=disable
JWT_SECRET=rahasia_jwt_token_super_aman
TMDB_API_KEY=api_key_tmdb_anda
```

---

## Jalankan di Lokal (Development)
```bash
# 1. Unduh dependensi
go mod tidy

# 2. Jalankan server
go run main.go
```
Server akan berjalan di `http://localhost:3000`.

---

## Deployment Produksi (VPS)

### 1. Build Executable Binary
```bash
go build -o cinelog-api main.go
```

### 2. Jalankan dengan Systemd / Background Service (Rekomendasi VPS)
Buat file service systemd di `/etc/systemd/system/cinelog-api.service`:
```ini
[Unit]
Description=Cinelog Go API Service
After=network.target postgresql.service

[Service]
Type=simple
User=root
WorkingDirectory=/path/to/cinelog/api
ExecStart=/path/to/cinelog/api/cinelog-api
Restart=always
RestartSec=5
EnvironmentFile=/path/to/cinelog/api/.env

[Install]
WantedBy=multi-user.target
```
Jalankan service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cinelog-api
```

### 3. Folder Asset lokal (VPS)
API secara otomatis menyimpan poster & backdrop yang di-download dari TMDB di folder `./uploads/posters` dan `./uploads/backdrops`. Pastikan folder ini memiliki izin akses tulis (*write permission*).

---

## Dokumentasi Endpoint
- `POST /api/auth/register` - Pendaftaran user
- `POST /api/auth/login` - Login & dapatkan token JWT
- `GET /api/search?q=query&type=all|movie|tv` - Search film/series dari TMDB
- `GET /api/user/watchlist` - Ambil watchlist user (Protected)
- `POST /api/user/watchlist` - Tambah film ke watchlist & simpan poster ke VPS (Protected)
- `PUT /api/user/watchlist/:id` - Edit rating / status / catatan (Protected)
- `DELETE /api/user/watchlist/:id` - Hapus tontonan (Protected)
- `GET /uploads/*` - Static image server (Media poster & backdrop)
