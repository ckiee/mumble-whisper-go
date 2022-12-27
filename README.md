# mumble-whisper-go
OpenAI whisper transcription bot for Mumble.

## reqs

- [`whisper.cpp`](https://github.com/ggerganov/whisper.cpp/) (i'm on `ac521a566ea6a79ba968c30101140db9f65d187b "ggml : simplify the SIMD code (#324)"`)
- go (i'm on 1.19.3)
- openssl
- (see `go.mod`)

## get started

``` shellsession
# (NixOS: Whisper dependency config)
nix-shell -p
export NIX_LDFLAGS="${NIX_LDFLAGS}-L /home/ckie/git/whisper.cpp -rpath /home/ckie/git/whisper.cpp"
export NIX_CFLAGS_COMPILE="${NIX_CFLAGS_COMPILE}-I /home/ckie/git/whisper.cpp"
# (Generate cert)
$ openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 365 -nodes
# (Run)
$ go mod download
$ go run main.go
# (✨)
```
