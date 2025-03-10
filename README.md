# Go Streamer

A simple video streaming server written in Go that converts uploaded videos into HLS and DASH formats for adaptive streaming.

## Features

- Video upload with drag-and-drop support
- Automatic conversion to HLS and DASH streaming formats
- Web-based video player with HLS and DASH playback
- Support for multiple video formats (mp4, avi, mov, mkv, wmv, flv, webm)
- Modern responsive UI using Tailwind CSS
- Real-time upload progress tracking
- Video library management

## Prerequisites

- Go 1.16 or later
- FFmpeg

## Setup Development Environment

1. Fork and Clone the repository:
```bash
git clone https://github.com/yourusername/go-streamer.git
cd go-streamer
```

2. Install FFmpeg:
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install ffmpeg

# macOS
brew install ffmpeg
```

3. Install Go dependencies:
```bash
go mod download
```

## Usage

1. Start the server:
```bash
go run main.go
```

2. Open your browser and navigate to:
```
http://localhost:5000
```

3. Upload videos using the web interface and they will be automatically converted to HLS and DASH formats.

## Project Structure

```
go-streamer/
├── main.go           # Main server code
├── templates/        # HTML templates
│   ├── index.html   # Upload and video list page
│   └── player.html  # Video player page
├── uploads/         # Temporary storage for uploaded files
└── streams/         # Converted video streams
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
