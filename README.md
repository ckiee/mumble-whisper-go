# mumble-whisper-go
OpenAI whisper transcription bot for Mumble.
## get started

``` shellsession
# (NixOS: Whisper dependency config)
nix-shell -p
export NIX_LDFLAGS="${NIX_LDFLAGS}-L /home/ckie/git/whisper.cpp -rpath /home/ckie/git/whisper.cpp"
export NIX_CFLAGS_COMPILE="${NIX_CFLAGS_COMPILE}-I /home/ckie/git/whisper.cpp"
# (Generate cert)
$ openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 365 -nodes
# (Run)
$ go get
$ go run main.go
# (✨)
```
