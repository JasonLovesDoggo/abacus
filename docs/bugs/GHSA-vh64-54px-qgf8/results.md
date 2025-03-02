
# After malicious script
```bash
❯ curl localhost:8080/stream/stream/eee/ -vvv
23:01:20.486586 [0-x] == Info: [READ] client_reset, clear readers
23:01:20.489178 [0-0] == Info: Host localhost:8080 was resolved.
23:01:20.491295 [0-0] == Info: IPv6: ::1
23:01:20.492584 [0-0] == Info: IPv4: 127.0.0.1
23:01:20.494114 [0-0] == Info: [SETUP] added
23:01:20.495946 [0-0] == Info:   Trying [::1]:8080...
23:01:20.498051 [0-0] == Info: Connected to localhost (::1) port 8080
23:01:20.500158 [0-0] == Info: using HTTP/1.x
23:01:20.502118 [0-0] => Send header, 96 bytes (0x60)
0000: GET /stream/stream/eee/ HTTP/1.1
0022: Host: localhost:8080
0038: User-Agent: curl/8.10.1
0051: Accept: */*
005e:
23:01:20.507492 [0-0] == Info: Request completely sent off
23:01:41.832824 [0-0] == Info: Recv failure: Connection was reset
23:01:41.835258 [0-0] == Info: [WRITE] cw-out done
23:01:41.836819 [0-0] == Info: closing connection #0
23:01:41.838722 [0-0] == Info: [SETUP] close
23:01:41.840378 [0-0] == Info: [SETUP] destroy
curl: (56) Recv failure: Connection was reset
~ via  v3.13.1 took 21s
```

# Before malicious script
```bash
❯ curl localhost:8080/stream/stream/eee/ -vvv
23:01:48.679494 [0-x] == Info: [READ] client_reset, clear readers
23:01:48.682091 [0-0] == Info: Host localhost:8080 was resolved.
23:01:48.684116 [0-0] == Info: IPv6: ::1
23:01:48.685387 [0-0] == Info: IPv4: 127.0.0.1
23:01:48.687056 [0-0] == Info: [SETUP] added
23:01:48.688790 [0-0] == Info:   Trying [::1]:8080...
23:01:48.690916 [0-0] == Info: Connected to localhost (::1) port 8080
23:01:48.692898 [0-0] == Info: using HTTP/1.x
23:01:48.694577 [0-0] => Send header, 96 bytes (0x60)
0000: GET /stream/stream/eee/ HTTP/1.1
0022: Host: localhost:8080
0038: User-Agent: curl/8.10.1
0051: Accept: */*
005e:
23:01:48.699693 [0-0] == Info: Request completely sent off
23:01:48.722188 [0-0] <= Recv header, 17 bytes (0x11)
0000: HTTP/1.1 200 OK
23:01:48.724437 [0-0] == Info: [WRITE] cw_out, wrote 17 header bytes -> 17
23:01:48.726908 [0-0] == Info: [WRITE] download_write header(type=c, blen=17) -> 0
23:01:48.729546 [0-0] == Info: [WRITE] client_write(type=c, len=17) -> 0
23:01:48.731849 [0-0] <= Recv header, 25 bytes (0x19)
0000: Cache-Control: no-cache
23:01:48.734304 [0-0] == Info: [WRITE] header_collect pushed(type=1, len=25) -> 0
23:01:48.736960 [0-0] == Info: [WRITE] cw_out, wrote 25 header bytes -> 25
23:01:48.739246 [0-0] == Info: [WRITE] download_write header(type=4, blen=25) -> 0
23:01:48.741791 [0-0] == Info: [WRITE] client_write(type=4, len=25) -> 0
23:01:48.744142 [0-0] <= Recv header, 24 bytes (0x18)
0000: Connection: keep-alive
23:01:48.746735 [0-0] == Info: [WRITE] header_collect pushed(type=1, len=24) -> 0
23:01:48.749113 [0-0] == Info: [WRITE] cw_out, wrote 24 header bytes -> 24
23:01:48.751155 [0-0] == Info: [WRITE] download_write header(type=4, blen=24) -> 0
23:01:48.753506 [0-0] == Info: [WRITE] client_write(type=4, len=24) -> 0
23:01:48.755627 [0-0] <= Recv header, 33 bytes (0x21)
0000: Content-Type: text/event-stream
23:01:48.758653 [0-0] == Info: [WRITE] header_collect pushed(type=1, len=33) -> 0
23:01:48.761138 [0-0] == Info: [WRITE] cw_out, wrote 33 header bytes -> 33
23:01:48.763515 [0-0] == Info: [WRITE] download_write header(type=4, blen=33) -> 0
23:01:48.766179 [0-0] == Info: [WRITE] client_write(type=4, len=33) -> 0
23:01:48.768501 [0-0] <= Recv header, 37 bytes (0x25)
0000: Date: Sun, 02 Mar 2025 04:01:48 GMT
23:01:48.771446 [0-0] == Info: [WRITE] header_collect pushed(type=1, len=37) -> 0
23:01:48.773906 [0-0] == Info: [WRITE] cw_out, wrote 37 header bytes -> 37
23:01:48.776037 [0-0] == Info: [WRITE] download_write header(type=4, blen=37) -> 0
23:01:48.778591 [0-0] == Info: [WRITE] client_write(type=4, len=37) -> 0
23:01:48.780857 [0-0] == Info: [WRITE] looking for transfer decoder: chunked
23:01:48.783072 [0-0] == Info: [WRITE] added transfer decoder chunked -> 0
23:01:48.785259 [0-0] <= Recv header, 28 bytes (0x1c)
0000: Transfer-Encoding: chunked
23:01:48.788082 [0-0] == Info: [WRITE] header_collect pushed(type=1, len=28) -> 0
23:01:48.790781 [0-0] == Info: [WRITE] cw_out, wrote 28 header bytes -> 28
23:01:48.792926 [0-0] == Info: [WRITE] download_write header(type=4, blen=28) -> 0
23:01:48.795653 [0-0] == Info: [WRITE] client_write(type=4, len=28) -> 0
23:01:48.797787 [0-0] <= Recv header, 2 bytes (0x2)
0000:
23:01:48.799552 [0-0] == Info: [WRITE] header_collect pushed(type=1, len=2) -> 0
23:01:48.802100 [0-0] == Info: [WRITE] cw_out, wrote 2 header bytes -> 2
23:01:48.804303 [0-0] == Info: [WRITE] download_write header(type=4, blen=2) -> 0
23:01:48.806762 [0-0] == Info: [WRITE] client_write(type=4, len=2) -> 0
23:01:48.808821 [0-0] <= Recv data, 26 bytes (0x1a)
0000: 14
0004: data: {"value":31}..
23:01:48.811317 [0-0] == Info: [WRITE] http_chunked, chunk start of 20 bytes
data: {"value":31}

23:01:48.813647 [0-0] == Info: [WRITE] cw_out, wrote 20 body bytes -> 20
23:01:48.815844 [0-0] == Info: [WRITE] download_write body(type=1, blen=20) -> 0
23:01:48.818394 [0-0] == Info: [WRITE] http_chunked, write 20 body bytes, 0 bytes in chunk remain
23:01:48.821546 [0-0] == Info: [WRITE] client_write(type=1, len=26) -> 0
23:01:48.823790 [0-0] == Info: [WRITE] xfer_write_resp(len=192, eos=0) -> 0
```
