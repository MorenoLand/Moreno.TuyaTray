# Protocol notes

Verified against real protocol 3.3 bulbs (data point styles A and B) and a live Tuya account.

## LAN framing (TCP 6668)

```
00 00 55 AA | seq u32 | cmd u32 | len u32 | payload | crc32 u32 | 00 00 AA 55
```

- All integers are big endian. `len` is the payload length plus 8 (CRC and suffix). `crc32` is IEEE over everything before it.
- Responses begin with a 4-byte return code unless its high 24 bits are non-zero (pushes carry none).
- Payloads are JSON encrypted with AES-128-ECB, PKCS7 padded, keyed by the device local key (16 ASCII bytes).
- Control commands (0x07, 0x0D) prefix the ciphertext with `"3.3"` and 12 zero bytes. Status queries (0x0A) send the ciphertext bare. Responses may carry the same 15-byte header.

| Command | Id | Use |
|---|---|---|
| CONTROL | 0x07 | write data points |
| DP_QUERY | 0x0A | read all data points |
| CONTROL_NEW | 0x0D | read points on firmwares that reject DP_QUERY (22-character device ids) |

Request body: `{"gwId":id,"devId":id,"uid":id,"t":"<unix>","dps":{...}}`. Some firmwares answer DP_QUERY with `json obj data unvalid`; asking CONTROL_NEW with null values for the known points returns the state.

## Discovery

Devices broadcast roughly every 5 seconds: UDP 6666 (plaintext JSON frame) and UDP 6667 (frame encrypted with `md5("yGAdlopoPVldABfn")`). The JSON carries `gwId`, `ip` and `version`.

## Data points

| Function | Style A (dp 1-5) | Style B (dp 20-24) |
|---|---|---|
| power | 1 bool | 20 bool |
| mode | 2 `white`/`colour`/`scene`/`music` | 21 |
| brightness | 3, 25..255 | 22, 10..1000 |
| colour temperature | 4, 0..255 (warm to cool) | 23, 0..1000 |
| colour | 5, `RRGGBB` + `HHHH` `SS` `VV` (14 hex chars, S and V 0..255) | 24, `HHHH` `SSSS` `VVVV` (12 hex chars, S and V 0..1000) |

Plugs and switches use dp 1 as a bool. In colour mode brightness is the V component. Whether colour temperature exists is read from the cloud status codes (`temp_value`) or the presence of dp 4 or 23.

## Account link (device sharing)

This is the flow Home Assistant's Tuya integration uses; no developer account is involved.

1. `POST https://apigw.iotbing.com/v1.0/m/life/home-assistant/qrcode/tokens?clientid=<id>&usercode=<code>&schema=haauthorize` returns a token.
2. The user scans `tuyaSmart--qrLogin?token=<token>` with the Tuya app.
3. `GET .../qrcode/tokens/<token>?clientid=<id>&usercode=<code>` returns, once approved, `endpoint`, `terminal_id`, `access_token`, `refresh_token`, `uid` and `expire_time`.

The client id is Home Assistant's public constant, `HA_3y9q4ak7g4ephrvke`.

### Signed requests to the returned endpoint

- `rid` is a random UUID-like string. `hash_key = md5hex(rid + refresh_token)`. `secret = hex(hmac_sha256(key=rid, msg=hash_key))[:16]`.
- Query parameters and JSON bodies are replaced by `encdata`: the JSON encrypted with AES-128-GCM using `secret` as the key and a random 12-character nonce, encoded as `base64(nonce) + base64(ciphertext||tag)`.
- Headers: `X-appKey`, `X-requestId`, `X-sid` (empty), `X-time` (ms), `X-token` (access token), `X-sign`.
- `X-sign = hex(hmac_sha256(key=hash_key, msg))` where `msg` is the non-empty headers joined as `X-appKey=..||X-requestId=..||X-time=..||X-token=..` followed directly by the encrypted query and body strings.
- Responses carry `result` as a base64 string encrypted the same way with the same `secret`.

### Endpoints used

| Purpose | Request |
|---|---|
| homes | `GET /v1.0/m/life/users/homes` |
| devices with local keys and status | `GET /v1.0/m/life/ha/home/devices` with `homeId` |
| room of a device | `GET /v1.0/m/thing/ha/<id>/room` |
| cloud commands | `POST /v1.1/m/thing/<id>/commands` with `{"commands":[{"code":..,"value":..}]}` |
| token refresh | `GET /v1.0/m/token/<refresh_token>` |

Cloud command codes: `switch_led` or `switch_1`, `work_mode`, `bright_value`, `temp_value`, `colour_data` (suffix `_v2` on style B devices; colour value is `{"h","s","v"}`).

## Home design file

The home view is a list of boxes, discs, posts, quads and lines in a simple isometric space. `x` runs along the front-left wall, `y` along the back-right wall and `z` is height, all in metres. A plan has `size` `[width, depth, wall height]`, `slots` (light spots with a `room` tag and `x`,`y`,`z`), `floors`, `walls`, `windows` and `items`. Each item has a sort key `k` `[x, y, w, d]` and a list of parts (`solid`, `box`, `part`, `disc`, `post`, `quad`, `line`). Colours are `[r, g, b]` and are blended toward night by the lit level of the part's `room`. Set `upper` to a room tag to add a raised area that appears only when a light is placed there.

Save a plan as `house.json` next to `settings.json`, or PUT it to `/plan/custom` on the local API. Plans are validated and limited in size.
