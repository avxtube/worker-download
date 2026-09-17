# Worker Download

Queue-based download worker สำหรับ AVXTUBE — claim งานจาก `video_process`, ดาวน์โหลด/ประมวลผลวิดีโอ แล้ว stage ผลลัพธ์เข้า temp storage เพื่อส่งต่อให้ ingest pipeline

> แทนที่ `server-download` เดิมที่ scan หาไฟล์เอง — ตัวนี้รับงานจากคิวอย่างเดียว

## Features

- **Queue-based** — atomic claim จาก `video_process` (pending → processing) ตาม priority ไม่มีทางแย่งงานกัน
- **Sources** — storage/upload-server ingest, `metadata.playlists`, direct URL และ scraper fallback จาก `metadata.source`
- **HLS headers** — ใช้ `metadata.source` เป็น Referer และ Origin ของทุก playlist/segment request
- **Temp ingest output** — อัปโหลดเข้า `video_process.tempStorageId` (หรือ temp stream storage ตาม priority) แล้วสร้าง ingest `destination=storage,status=uploaded,sourceType=processed`; file → `ready_original`
- **MP4 faststart gate** — ตรวจ top-level atom ว่า `moov` อยู่ก่อน `mdat` ก่อน upload ทุกงาน; ถ้าไม่ใช่จะ remux `+faststart` และตรวจซ้ำก่อนแทนไฟล์เดิม
- **Auto Retry + Backoff** — fail → กลับเป็น pending ใน doc เดิม (1m, 2m) ครบ 3 ครั้ง → failed ถาวร + file → `error`
- **Instant Cancel** — admin เซ็ต `status: cancelled` → watcher (5s) จุดระเบิด context → HTTP/ffmpeg/S3 หยุดทันที
- **Graceful Shutdown** — SIGTERM → คืนงานเข้าคิว (Release) + mark worker offline
- **Heartbeat** — รายงานเข้า `workers` ทุก 1 นาที (idle/busy/paused, disk ≥90% = paused + enable=false)
- **Disk-safe split** — ก่อนแยก video/audio/subtitle ต้องมีพื้นที่ว่างอย่างน้อยขนาด source + 512 MB; ถ้าพื้นที่ไม่พอจะลบ split output ที่ไม่สมบูรณ์และ requeue โดยไม่กิน retry
- **Early segment cleanup** — หลังรวม HLS และตรวจ `file_original.mp4` ผ่านแล้ว จะลบ source segments ทันทีเพื่อคืนพื้นที่ก่อนเริ่ม split
- **Realtime dashboard** — `:8885` แสดง CPU, RAM, disk/I/O และ progress งานของทุก instance ผ่าน SSE ทุก 1 วินาที (เปิดเว็บโดย worker `@1` ตัวเดียว)
- **Live process log** — กด `View log` ในแต่ละ job หรือเปิด `/log/<slug>.log` เพื่ออ่าน `logs/process/<slug>.log`
- **Realtime progress** — บันทึก `timeline`/`overallPercent` ทุก 1% แต่ process log ยังคง throttle ทุก 10%
- **Optional NVIDIA GPU** — ทดสอบ NVENC ด้วยการ encode จริงก่อนใช้กับงาน re-encode และ fallback เป็น `libx264` อัตโนมัติ; Dashboard แสดง GPU/VRAM/NVENC เมื่อมี `nvidia-smi`
- **Failure diagnostics** — เก็บ source, segment และ output ไว้ระหว่าง retry; ลบ work directory หลังสำเร็จ, cancelled หรือ retry ครบ และเก็บ process log แยกไว้ที่ `logs/process/<slug>.log`

## Requirements

- **FFmpeg** + **FFprobe** (ต้องอยู่ใน PATH)
- **NVIDIA driver + `nvidia-smi`** (ไม่บังคับ — ใช้สำหรับ NVENC และ GPU metrics)
- **MongoDB** (AVXTUBE platform database — replica set)
- **AVXTUBE node-api** รันอยู่ (enqueuer เติมคิว + reaper)

---

## Installation (Linux Server)

### One-line install

```bash
curl -fsSL https://raw.githubusercontent.com/avxtube/worker-download/main/install.sh | sudo -E bash -s -- \
    --database-url "mongodb+srv://user:pass@cluster.mongodb.net/platform" \
    -n 1
```

### Options

| Option | Default | คำอธิบาย |
|---|---|---|
| `-n, -w, --count` | `1` | จำนวน worker instances |
| `--database-url` | `""` | MongoDB connection string (`DATABASE_URL`) |
| `--storage-encryption-key` | inherited env or `""` | Secret เดียวกับ `STORAGE_ENCRYPTION_KEY`/`BETTER_AUTH_SECRET` ของ node-api สำหรับถอดรหัส S3 credentials |
| `--storage-id` | `""` | Local storage ID สำหรับ fallback เมื่อไม่มี S3 `storage + video` |
| `--storage-path` | `/home/files` | Local storage path |
| `--scraper-url` | `""` | Scraper API (ไม่ตั้ง = อ่านจาก `settings.url_scraping`) |
| `--dashboard-port` | `8885` | พอร์ต realtime dashboard (worker `@1` เป็นผู้เปิดเว็บ) |
| `--uninstall` | — | ถอนการติดตั้ง |

### After install

```bash
# ดู logs
journalctl -u "worker-download@*" -f

# ดู worker 1
journalctl -u "worker-download@1" -f

# Restart workers
for i in $(seq 1 2); do systemctl restart worker-download@$i; done

# เปิด dashboard (ต้องเปิด firewall/TCP 8885 หากเข้าจากภายนอก)
http://SERVER_IP:8885

# Stop workers (SIGTERM → คืนงานเข้าคิวก่อนปิด)
for i in $(seq 1 2); do systemctl stop worker-download@$i; done
```

---

## Download Latest Release

```bash
# Linux amd64
curl -L https://github.com/avxtube/worker-download/releases/latest/download/linux -o worker-download
chmod +x worker-download

# Linux ARM64
curl -L https://github.com/avxtube/worker-download/releases/latest/download/linux-arm64 -o worker-download
chmod +x worker-download
```

---

## Configuration (.env)

```env
# Required
DATABASE_URL=mongodb+srv://user:pass@cluster.mongodb.net/platform

# Required only when temp storage provider is S3
STORAGE_ENCRYPTION_KEY=same-secret-used-by-node-api

# Optional — Worker ID (default: download_hostname@1)
WORKER_ID=download_myhost@1

# Optional — Scraper URL (ไม่ตั้ง = อ่านจาก settings.url_scraping)
SCRAPER_URL=http://localhost:8081

# Optional — realtime monitor (default: 8885)
DASHBOARD_PORT=8885

# Output layout
MEDIA_LAYOUT=muxed

# Optional (default: .build/work)
WORK_DIR=.build/work

# Optional — log file (default: logs/worker-download.log)
LOG_PATH=logs/worker-download.log
```

---

## Development

```bash
git clone https://github.com/avxtube/worker-download.git
cd worker-download

# สร้าง .env แล้วใส่ DATABASE_URL

# Run
go run ./cmd

# Build (Windows exe + copy .env → .build/)
build.bat
```

---

## Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

GitHub Actions build + release อัตโนมัติ:
- `linux` — Linux amd64 binary
- `linux-arm64` — Linux ARM64 binary

---

## Architecture

```
AVXTUBE node-api                          worker-download (Go, ตัวนี้)
├── enqueuer (ทุก 20s)                    ├── heartbeat (ทุก 1m → workers)
│   slot ว่าง → files pending/queue       ├── job loop
│   → insert video_process pending        │   ResumeOwn → Claim (atomic, priority)
└── reaper                                │   → download → merge/encode → probe
    processing lease หมดอายุ               │   → upload temp storage
    → คืน pending                          │   → processed ingest + ready_original
                                          │   → Complete | RetryOrFail | Release
                                          └── cancel watcher (ทุก 5s ระหว่างมีงาน)
```

## Job Lifecycle

```
pending ──claim──▶ processing ──สำเร็จ──▶ completed
   ▲                   │
   │◀── retry (backoff 1m/2m, ≤3) ── fail
   │◀── Release (shutdown / disk เต็ม / reaper)
   │
   └── admin เซ็ต cancelled ──▶ หยุดทุก I/O ใน ≤5s และเก็บ work/log
       fail ครั้งที่ 3 ──▶ failed ถาวร + file → error
       completed ──▶ cleanup .build/work/<jobId>
```

## Collections Used

| Collection | การใช้งาน |
|---|---|
| `video_process` | คิวงาน — claim/settle/timeline (contract ตรงกับ AVXTUBE platform) |
| `workers` | heartbeat, สถานะ, system info |
| `files` | อ่าน `metadata.source/playlists`, update processing/ready_original/error |
| `ingests` | อ่าน source จาก `temporaryPath` หรือ `storageId+key`; สร้าง processed ingest |
| `storages` | อ่าน temp storage และ config provider ปัจจุบัน |
| `settings` | `download_config.enabled` (kill switch), `url_scraping` |

> ⚠ **Index ทั้งหมดเป็นของฝั่ง AVXTUBE platform (mongoose)** — repo นี้ไม่สร้าง index เอง
> ⚠ ค่า enum ทุกตัวใน `internal/core/enums/` ต้อง match กับ `platform/packages/core/src/enums/`
