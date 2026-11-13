# Filebrowser media image (GPU)

`Dockerfile.media` builds a media image on top of the Debian GPU FFmpeg image
(`gtstef/ffmpeg:<version>-gpu`, Track G in `gtsteffaniak/ffmpeg`). The
production Pi 4 setup stays on the static Alpine `Dockerfile` (V4L2).

## Prereq (sequencing)

The media image is built only **after** the FFmpeg GPU image is published
(`gtstef/ffmpeg:<version>-gpu`) and smoke-tested. The `FFMPEG_GPU_IMAGE`
build-arg pins the exact published tag; if the tag is not smoke-tested, do
not merge the media PR.

## Build

```bash
docker build -f _docker/Dockerfile.media \
  --build-arg FFMPEG_GPU_IMAGE=gtstef/ffmpeg:9.0.2-gpu \
  --build-arg VERSION=latest --build-arg REVISION=$(git rev-parse HEAD) \
  -t filebrowser-media:latest .
```

## Smoke

```bash
docker run --rm filebrowser-media:latest ffmpeg -hwaccels | grep -E "vaapi|cuda|vulkan|opencl"
docker run --rm --device=/dev/dri/renderD128 filebrowser-media:latest \
  -vaapi_device /dev/dri/renderD128 -f lavfi -i testsrc=size=640x480:rate=1 \
  -c:v h264_vaapi -t 1 -f null -
docker run --rm --gpus all filebrowser-media:latest \
  -f lavfi -i testsrc=size=640x480:rate=1 -c:v h264_nvenc -t 1 -f null -
```

## Compose

See `docker-compose.media.yaml` (intel/AMD and NVIDIA snippets; Pi stays
static).
