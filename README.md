# ~~mumble-whisper-go~~
# mumble-deepgram-go
~~OpenAI Whisper~~ [Deepgram](https://deepgram.com) transcription bot for Mumble.

## reqs

- an account on deepgram and an API key from it
- go (i'm on 1.19.4)
- openssl
- (see `go.mod`)

## get started

``` shellsession
# (Generate cert)
$ openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 365 -nodes
# (Run)
$ go mod download
$ go run main.go -server mumble.cyberia.club:64738 -certificate cert.pem -key key.pem -insecure -username $USER-bot -apikey $(cat .dg-api-key)
# (✨)
```

## nix
if you're using nix, you can enable the glorious direnv (perhaps in your home-manager config!)
along with lorri, run `direnv allow` and your direnv-compatible editor should pick up on the env
changes and use the gopls binary specified in `shell.nix` -- it also puts openssl and go in PATH.

## why didn't you like whisper

i did kinda like it but there's a bunch of post-processing to do and whisper.cpp 
runs on CPU-only, 4 threads or something.

i saw deepgram on a [zack freedman](https://www.youtube.com/@ZackFreedman/videos) youtube video
and the thing just fucking works except for little-endian u16s being a weird choice.

i hope soon enough we'll have something that fits nicely on a raspi and can do all the things with like, 40% resource util.

